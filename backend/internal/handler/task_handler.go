// Package handler exposes the task service over HTTP.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/example/task-management/backend/internal/domain"
	"github.com/example/task-management/backend/internal/service"
)

// TaskService is what the handler needs; *service.TaskService satisfies it.
type TaskService interface {
	List(ctx context.Context, p domain.ListParams) (*domain.TaskPage, error)
	Get(ctx context.Context, id int64) (*domain.Task, error)
	Create(ctx context.Context, in domain.TaskInput) (*domain.Task, error)
	Update(ctx context.Context, id int64, in domain.TaskInput) (*domain.Task, error)
	Delete(ctx context.Context, id int64) error
}

type TaskHandler struct{ svc TaskService }

func NewTaskHandler(svc TaskService) *TaskHandler { return &TaskHandler{svc: svc} }

// NewRouter wires all routes. Kept separate from main so tests get the exact
// production router.
func NewRouter(h *TaskHandler) *gin.Engine {
	r := gin.New()
	r.HandleMethodNotAllowed = true
	r.Use(gin.Logger(), recovery(), cors())

	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	api := r.Group("/api")
	api.GET("/tasks", h.list)
	api.POST("/tasks", h.create)
	api.GET("/tasks/:id", h.get)
	api.PUT("/tasks/:id", h.update)
	api.DELETE("/tasks/:id", h.delete)

	r.NoRoute(func(c *gin.Context) { writeError(c, http.StatusNotFound, CodeNotFound, "route not found") })
	r.NoMethod(func(c *gin.Context) {
		writeError(c, http.StatusMethodNotAllowed, CodeMethodNotAllowed, "method not allowed")
	})
	return r
}

// taskRequest uses pointers so we can tell "missing" from "empty" and return
// precise validation messages.
type taskRequest struct {
	Title       *string      `json:"title"`
	Description *string      `json:"description"`
	Status      *string      `json:"status"`
	Assignee    *string      `json:"assignee"`
	DueDate     *domain.Date `json:"due_date"`
}

func (h *TaskHandler) list(c *gin.Context) {
	p, err := service.NormalizeListQuery(service.RawListQuery{
		Status:   c.Query("status"),
		Keyword:  c.Query("keyword"),
		Assignee: c.Query("assignee"),
		Page:     c.Query("page"),
		Limit:    c.Query("limit"),
		Sort:     c.Query("sort"),
	})
	if err != nil {
		respondError(c, err)
		return
	}
	page, err := h.svc.List(c.Request.Context(), p)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *TaskHandler) get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	t, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": t})
}

func (h *TaskHandler) create(c *gin.Context) {
	in, ok := bindTask(c)
	if !ok {
		return
	}
	t, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": t})
}

func (h *TaskHandler) update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	in, ok := bindTask(c)
	if !ok {
		return
	}
	t, err := h.svc.Update(c.Request.Context(), id, in)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": t})
}

func (h *TaskHandler) delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(c, http.StatusBadRequest, CodeValidation, "id must be a positive integer")
		return 0, false
	}
	return id, true
}

func bindTask(c *gin.Context) (domain.TaskInput, bool) {
	var req taskRequest
	dec := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		msg := "invalid request body: " + err.Error()
		if errors.Is(err, io.EOF) {
			msg = "request body is required"
		}
		writeError(c, http.StatusBadRequest, CodeInvalidJSON, msg)
		return domain.TaskInput{}, false
	}
	if req.Title == nil {
		writeError(c, http.StatusBadRequest, CodeValidation, "title is required")
		return domain.TaskInput{}, false
	}
	in := domain.TaskInput{
		Title:       *req.Title,
		Description: req.Description,
		Assignee:    req.Assignee,
		DueDate:     req.DueDate,
	}
	if req.Status != nil {
		in.Status = domain.Status(*req.Status)
	}
	return in, true
}

// cors allows the Expo web/dev client to call the API from another origin.
func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
