package main

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type ItemStatus int

const (
	ItemStatusDoing ItemStatus = iota
	ItemStatusDone
	ItemStatusDeleted
)

var allItemStatuses = [3]string{
	"Doing",
	"Done",
	"Deleted",
}

func (item *ItemStatus) String() string {
	return allItemStatuses[*item]
}

func parseStr2ItemStatus(s string) (ItemStatus, error) {
	for i := range allItemStatuses {
		if allItemStatuses[i] == s {
			return ItemStatus(i), nil
		}
	}

	return ItemStatus(0), errors.New("invalid status string")
}

func (item *ItemStatus) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprintf("fail to scan data from sql: %v", value))
	}

	v, err := parseStr2ItemStatus(string(bytes))
	if err != nil {
		return errors.New(fmt.Sprintf("fail to scan data from sql : %v", value))
	}
	*item = v

	return nil
}

func (item *ItemStatus) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", item.String())), nil
}

func (item *ItemStatus) Value() (driver.Value, error) {
	if item == nil {
		return nil, nil
	}

	return item.String(), nil
}

func (item *ItemStatus) UnmarshalJSON(data []byte) error {
	str := strings.ReplaceAll(string(data), "\"", "")
	itemValue, err := parseStr2ItemStatus(str)
	if err != nil {
		return err
	}
	*item = itemValue

	return nil
}

type TodoItem struct {
	Id          uint        `json:"id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Status      *ItemStatus `json:"status"`
	CreatedAt   *time.Time  `json:"created_at"`
	UpdatedAt   *time.Time  `json:"updated_at"`
}

type TodoItemCreation struct {
	Id          uint        `json:"-" gorm:"column:id;"`
	Title       string      `json:"title" gorm:"column:title;"`
	Description string      `json:"description" gorm:"column:description;"`
	Status      *ItemStatus `json:"status" gorm:"column:status;"`
}

type TodoItemUpdate struct {
	Title       *string `json:"title" gorm:"column:title;"`
	Description *string `json:"description" gorm:"column:description;"`
	Status      *string `json:"status" gorm:"column:status;"`
}

func (TodoItem) TableName() string {
	return "todo_items"
}

func (TodoItemCreation) TableName() string {
	return TodoItem{}.TableName()
}

func (TodoItemUpdate) TableName() string {
	return TodoItem{}.TableName()
}

type Paging struct {
	Page  int   `json:"page" form:"page"`
	Limit int   `json:"limit" form:"limit"`
	Total int64 `json:"total" form:"-"`
}

func (p *Paging) Process() {
	if p.Page <= 0 {
		p.Page = 1
	}

	if p.Limit <= 0 || p.Limit >= 100 {
		p.Limit = 10
	}
}

func main() {
	dsn := os.Getenv("DATABASE_URL")

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})

	if err != nil {
		log.Fatalln(err)
	}

	router := gin.Default()

	v1 := router.Group("/v1")
	{
		items := v1.Group("/items")
		{
			items.POST("/", CreateItem(db))
			items.GET("/", ListItem(db))
			items.GET("/:id", GetItem(db))
			items.PATCH("/:id", UpdateItem(db))
			items.DELETE("/:id", DeleteItem(db))
		}
	}

	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "ssss",
		})
	})

	router.Run()
}

func CreateItem(db *gorm.DB) func(c *gin.Context) {
	return func(c *gin.Context) {
		var data TodoItemCreation
		if err := c.ShouldBind(&data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})

			return
		}

		if err := db.Create(&data).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})

			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": data.Id,
		})
	}
}

func GetItem(db *gorm.DB) func(c *gin.Context) {
	return func(c *gin.Context) {
		var data TodoItem
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})

			return
		}

		if err := db.Where("id = ?", id).First(&data).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})

			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": data,
		})
	}
}

func UpdateItem(db *gorm.DB) func(c *gin.Context) {
	return func(c *gin.Context) {
		var data TodoItemUpdate

		id, err := strconv.Atoi(c.Param("id"))

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})

			return
		}

		if err := c.ShouldBind(&data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})

			return
		}

		if err := db.Where("id = ?", id).Updates(&data).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})

			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": true,
		})
	}
}

func DeleteItem(db *gorm.DB) func(c *gin.Context) {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})

			return
		}

		//if err := db.Table(TodoItem{}.TableName()).Where("id = ?", id).Delete(nil).Error; err != nil {
		if err := db.Table(TodoItem{}.TableName()).Where("id = ?", id).Updates(map[string]interface{}{
			"status": "Deleted",
		}).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})

			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": true,
		})
	}
}

func ListItem(db *gorm.DB) func(c *gin.Context) {
	return func(c *gin.Context) {
		var paging Paging

		if err := c.ShouldBind(&paging); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})

			return
		}

		paging.Process()

		if err := db.Table(TodoItem{}.TableName()).Where("status <> ?", "Deleted").Count(&paging.Total).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})

			return
		}

		var result []TodoItem

		if err := db.Order("id desc").
			Offset((paging.Page-1)*paging.Limit).
			Where("status <> ?", "Deleted").
			Limit(paging.Limit).
			Find(&result).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})

			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data":   result,
			"paging": paging,
		})
	}
}
