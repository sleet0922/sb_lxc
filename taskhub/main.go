// TaskHub - 轻量任务管理系统
// 提供 REST API: 创建/列表/完成/删除任务，MariaDB 持久化 + Redis 缓存统计
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"context"
)

type Task struct {
	ID        int64  `json:"id"`
	Title     string `json:"title" binding:"required"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type Stats struct {
	Total   int64 `json:"total"`
	Done    int64 `json:"done"`
	Pending int64 `json:"pending"`
}

var (
	db    *sql.DB
	rdb   *redis.Client
	cacheTTL = 30 * time.Second
)

func main() {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "taskhub:taskhub123@tcp(127.0.0.1:3306)/taskhub?charset=utf8mb4&parseTime=true"
	}
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "127.0.0.1:6379"
	}
	listen := os.Getenv("LISTEN_ADDR")
	if listen == "" {
		listen = ":8080"
	}

	// 等待 MariaDB 就绪
	if err := waitForDB(dsn, 30); err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}

	// 配置 MySQL 参数
	mysql.SetLogger(log.New(os.Stderr, "[mysql] ", log.LstdFlags))
	db, _ = sql.Open("mysql", dsn)
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	if err := db.Ping(); err != nil {
		log.Fatalf("数据库 ping 失败: %v", err)
	}

	// 初始化表
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS tasks (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		title VARCHAR(255) NOT NULL,
		status ENUM('pending','done') DEFAULT 'pending',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	) CHARACTER SET utf8mb4`); err != nil {
		log.Fatalf("建表失败: %v", err)
	}

	// 连接 Redis
	rdb = redis.NewClient(&redis.Options{Addr: redisAddr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Printf("⚠ Redis 连接失败 (降级运行): %v", err)
		rdb = nil
	}

	// Gin 路由
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now().Format(time.RFC3339)})
	})

	r.GET("/stats", getStats)

	api := r.Group("/api/v1")
	api.POST("/tasks", createTask)
	api.GET("/tasks", listTasks)
	api.PUT("/tasks/:id/done", doneTask)
	api.DELETE("/tasks/:id", deleteTask)

	log.Printf("TaskHub 启动，监听 %s", listen)
	if err := r.Run(listen); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
}

func waitForDB(dsn string, maxWait int) error {
	for i := 0; i < maxWait; i++ {
		tmp, err := sql.Open("mysql", dsn)
		if err == nil {
			if err = tmp.Ping(); err == nil {
				tmp.Close()
				return nil
			}
			tmp.Close()
		}
		log.Printf("等待数据库 (%d/%d)...", i+1, maxWait)
		time.Sleep(time.Second)
	}
	return fmt.Errorf("超过 %d 秒未连接到数据库", maxWait)
}

func getStats(c *gin.Context) {
	cacheKey := "taskhub:stats"
	if rdb != nil {
		if val, err := rdb.Get(context.Background(), cacheKey).Result(); err == nil {
			var s Stats
			if json.Unmarshal([]byte(val), &s) == nil {
				c.JSON(http.StatusOK, gin.H{"source": "cache", "stats": s})
				return
			}
		}
	}

	var s Stats
	if err := db.QueryRow("SELECT COUNT(*) FROM tasks").Scan(&s.Total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM tasks WHERE status='done'").Scan(&s.Done); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.Pending = s.Total - s.Done

	if rdb != nil {
		if data, err := json.Marshal(s); err == nil {
			rdb.Set(context.Background(), cacheKey, data, cacheTTL)
		}
	}
	c.JSON(http.StatusOK, gin.H{"source": "db", "stats": s})
}

func createTask(c *gin.Context) {
	var t Task
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := db.Exec("INSERT INTO tasks (title) VALUES (?)", t.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	t.ID, _ = res.LastInsertId()
	t.Status = "pending"
	t.CreatedAt = time.Now().Format(time.RFC3339)
	invalidateCache()
	c.JSON(http.StatusCreated, t)
}

func listTasks(c *gin.Context) {
	rows, err := db.Query("SELECT id, title, status, created_at FROM tasks ORDER BY id DESC LIMIT 100")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		var createdAt sql.NullTime
		if err := rows.Scan(&t.ID, &t.Title, &t.Status, &createdAt); err == nil {
			if createdAt.Valid {
				t.CreatedAt = createdAt.Time.Format(time.RFC3339)
			}
			tasks = append(tasks, t)
		}
	}
	if tasks == nil {
		tasks = []Task{}
	}
	c.JSON(http.StatusOK, tasks)
}

func doneTask(c *gin.Context) {
	id := c.Param("id")
	res, err := db.Exec("UPDATE tasks SET status='done' WHERE id=?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}
	invalidateCache()
	c.JSON(http.StatusOK, gin.H{"id": id, "status": "done"})
}

func deleteTask(c *gin.Context) {
	id := c.Param("id")
	res, err := db.Exec("DELETE FROM tasks WHERE id=?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}
	invalidateCache()
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

func invalidateCache() {
	if rdb != nil {
		rdb.Del(context.Background(), "taskhub:stats")
	}
}
