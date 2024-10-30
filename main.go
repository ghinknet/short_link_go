package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

var (
	db       *sql.DB
	fieldMap = map[rune]int{
		'A': 0, 'a': 1, 'B': 2, 'b': 3,
		'C': 4, 'c': 5, 'D': 6, 'd': 7,
		'1': 8, 'E': 9, 'e': 10, 'F': 11,
		'f': 12, 'G': 13, 'g': 14, 'H': 15,
		'h': 16, '2': 17, 'I': 18, 'i': 19,
		'J': 20, 'j': 21, 'K': 22, 'k': 23,
		'L': 24, 'l': 25, '3': 26, 'M': 27,
		'm': 28, 'N': 29, 'n': 30, 'O': 31,
		'o': 32, 'P': 33, 'p': 34, '4': 35,
		'Q': 36, 'q': 37, 'R': 38, 'r': 39,
		'S': 40, 's': 41, 'T': 42, 't': 43,
		'5': 44, 'U': 45, 'u': 46, 'V': 47,
		'v': 48, 'W': 49, 'w': 50, 'X': 51,
		'x': 52, '6': 53, 'Y': 54, 'y': 55,
		'Z': 56, 'z': 57, '7': 58, '8': 59,
		'9': 60, '0': 61,
	}
)

type Config struct {
	DB     DBConfig `json:"DB"`
	KEYS   []string `json:"KEYS"`
	LISTEN []string `json:"LISTEN"`
	DEBUG  bool     `json:"DEBUG"`
}

type DBConfig struct {
	Host     string `json:"host"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

func loadConfig() (Config, error) {
	file, err := os.Open("config.json")
	if err != nil {
		return Config{}, err
	}
	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			panic(err)
		}
	}(file)

	var config Config
	if err = json.NewDecoder(file).Decode(&config); err != nil {
		return Config{}, err
	}
	return config, nil
}

func initDB(dbConfig DBConfig) error {
	var err error
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s", dbConfig.User, dbConfig.Password, dbConfig.Host, dbConfig.Database)
	db, err = sql.Open("mysql", dsn)
	return err
}

func read404Page() (string, error) {
	data, err := os.ReadFile("404.html")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func main() {
	config, err := loadConfig()
	if err != nil {
		panic(err)
	}

	if err = initDB(config.DB); err != nil {
		panic(err)
	}
	defer func(db *sql.DB) {
		err = db.Close()
		if err != nil {
			fmt.Println(err.Error())
			return
		}
	}(db)

	router := gin.Default()

	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "https://k76u22n4gd.apifox.cn")
	})


	router.GET("/:link_id", func(c *gin.Context) {
		linkID := c.Param("link_id")

		for _, char := range linkID {
			if _, exists := fieldMap[char]; !exists {
				page, _ := read404Page()
				c.Data(http.StatusNotFound, "text/html", []byte(page))
				return
			}
		}

		linkIDConverted := 0
		for i := 0; i < len(linkID); i++ {
			char := rune(linkID[len(linkID)-1-i])
			linkIDConverted += fieldMap[char] * intPow(62, i)
		}

		var link string
		var validity sql.NullInt64
		err = db.QueryRow("SELECT link, validity FROM links WHERE id=?", linkIDConverted).Scan(&link, &validity)
		if err != nil || link == "" {
			page, _ := read404Page()
			c.Data(http.StatusNotFound, "text/html", []byte(page))
			return
		}

		if validity.Valid && validity.Int64 < time.Now().Unix() {
			removeLink(linkIDConverted)
			page, _ := read404Page()
			c.Data(http.StatusNotFound, "text/html", []byte(page))
			return
		}

		c.Redirect(http.StatusFound, link)
	})

	router.POST("/", func(c *gin.Context) {
		key := c.PostForm("key")
		link := c.PostForm("link")
		validity := c.PostForm("validity")

		if key == "" || link == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "bad field(s)", "content": ""})
			return
		}

		if !contains(config.KEYS, key) {
			c.JSON(http.StatusForbidden, gin.H{"ok": false, "message": "forbidden", "content": ""})
			return
		}

		var validityInt *int64
		if validity != "" {
			v, err := strconv.ParseInt(validity, 10, 64)
			if err != nil || v <= time.Now().Unix() {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "bad field(s)", "content": ""})
				return
			}
			validityInt = &v
		}

		var linkIDRandom string
		var linkIDConverted int
		for {
			linkIDRandom = randomString(6)
			linkIDConverted = 0
			for i := 0; i < len(linkIDRandom); i++ {
				char := rune(linkIDRandom[len(linkIDRandom)-1-i])
				linkIDConverted += fieldMap[char] * intPow(62, i)
			}

			var existingLink string
			err = db.QueryRow("SELECT link FROM links WHERE id=?", linkIDConverted).Scan(&existingLink)
			if err != nil && !errors.Is(sql.ErrNoRows, err) {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "error", "content": ""})
				return
			}
			if existingLink == "" {
				break
			}
		}

		_, err = db.Exec("INSERT INTO links VALUES (?, ?, ?)", linkIDConverted, link, validityInt)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "error", "content": ""})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "successful", "content": linkIDRandom})
	})

	router.PATCH("/", func(c *gin.Context) {
		config, err = loadConfig()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "error"})
			return
		}

		if err = initDB(config.DB); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "successful"})
	})

	router.Run(fmt.Sprintf("%s:%s", config.LISTEN[0], config.LISTEN[1]))
}

func intPow(base, exp int) int {
	result := 1
	for exp > 0 {
		result *= base
		exp--
	}
	return result
}

func randomString(length int) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]rune, length)
	for i := range result {
		result[i] = rune(chars[rand.Intn(len(chars))])
	}
	return string(result)
}

func contains(slice []string, item string) bool {
	for _, a := range slice {
		if a == item {
			return true
		}
	}
	return false
}

func removeLink(id int) {
	_, _ = db.Exec("DELETE FROM links WHERE id=?", id)
}
