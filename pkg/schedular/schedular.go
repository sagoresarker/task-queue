package schedular

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/pgtype"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/sagoresarker/task-queue/pkg/common"
)

type CommandRequest struct {
	Command     string `json:"command"`
	ScheduledAt string `json:"scheduled_at"`
}

type Task struct {
	Id          string
	Command     string
	ScheduledAt pgtype.Timestamp
	PickedAt    pgtype.Timestamp
	StartedAt   pgtype.Timestamp
	CompletedAt pgtype.Timestamp
	FailedAt    pgtype.Timestamp
}

// schedule server represents an http server that manage tasks.
type SchedularServer struct {
	serverPort         string
	dbConnectionString string
	dbPool             *pgxpool.Pool
	ctx                context.Context
	cancel             context.CancelFunc
	httpServer         *http.Server
}

// NewServer create and returns a new instance of SchedularServer.
func NewServer(port string, dbConnectionString string) *SchedularServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &SchedularServer{
		serverPort:         port,
		dbConnectionString: dbConnectionString,
		ctx:                ctx,
		cancel:             cancel,
	}
}

// Start starts the server.
func (s *SchedularServer) Start() error {
	var err error
	s.dbPool, err = common.ConnectToDatabase(s.ctx, s.dbConnectionString)
	if err != nil {
		return err
	}

	http.HandleFunc("/schedule", s.handleScheduleTask)
	http.HandleFunc("/status", s.handleGetTaskStatus)

	s.httpServer = &http.Server{
		Addr: s.serverPort,
	}

	log.Printf("Server started at port %s\n", s.serverPort)

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil {
			log.Printf("Error starting server: %s\n", err)
		}
	}()

	return s.awaitShutdown()

}

func (s *SchedularServer) handleScheduleTask(w http.ResponseWriter, r *http.Request) {

	if r.Method != "POST" {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var commandReq CommandRequest
	if err := json.NewDecoder(r.Body).Decode(&commandReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Received schedule request: %+v", commandReq)

	scheduledTime, err := time.Parse(time.RFC3339, commandReq.ScheduledAt)
	if err != nil {
		http.Error(w, "Invalid date format. Use ISO 8601 format.", http.StatusBadRequest)

		return
	}

	unixTimestamp := time.Unix(scheduledTime.Unix(), 0)

	taskId, err := s.insertTaskIntoDB(context.Background(), Task{Command: commandReq.Command, ScheduledAt: pgtype.Timestamp{Time: unixTimestamp}})

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to submit task. Error: %s", err.Error()),
			http.StatusInternalServerError)
		return
	}

	response := struct {
		Command     string `json:"command"`
		ScheduledAt int64  `json:"scheduled_at"`
		TaskID      string `json:"task_id"`
	}{
		Command:     commandReq.Command,
		ScheduledAt: unixTimestamp.Unix(),
		TaskID:      taskId,
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonResponse)
}

func (s *SchedularServer) handleGetTaskStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	taskId := r.URL.Query().Get("task_id")
	if taskId == "" {
		http.Error(w, "Task id is required", http.StatusBadRequest)
		return
	}

	log.Printf("Received get task status request for task id: %s\n", taskId)

	// query the database to get the task status
	var task Task
	err := s.dbPool.QueryRow(context.Background(), "SELECT * FROM tasks WHERE id = $1", taskId).Scan(&task.Id, &task.Command, &task.ScheduledAt, &task.PickedAt, &task.StartedAt, &task.CompletedAt, &task.FailedAt)
	if err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	response := struct {
		TaskId      string `json:"task_id"`
		Command     string `json:"command"`
		ScheduledAt string `json:"schedule_at"`
		PickedAt    string `json:"picked_at"`
		StartedAt   string `json:"started_at"`
		CompletedAt string `json:"completed_at"`
		FailedAt    string `json:"failed_at"`
	}{
		TaskId:      task.Id,
		Command:     task.Command,
		ScheduledAt: "",
		PickedAt:    "",
		StartedAt:   "",
		CompletedAt: "",
		FailedAt:    "",
	}

	// set the schedule time if it is not null
	if task.ScheduledAt.Status == 2 {
		response.ScheduledAt = task.ScheduledAt.Time.String()
	}

	// set the picked time if it is not null
	if task.PickedAt.Status == 2 {
		response.PickedAt = task.PickedAt.Time.String()
	}

	// set the started time if it is not null
	if task.StartedAt.Status == 2 {
		response.StartedAt = task.StartedAt.Time.String()
	}

	// set the completed time if it is not null
	if task.CompletedAt.Status == 2 {
		response.CompletedAt = task.CompletedAt.Time.String()
	}

	// set the failed time if it is not null
	if task.FailedAt.Status == 2 {
		response.FailedAt = task.FailedAt.Time.String()
	}

	// convert the response to json
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonResponse)

}

func (s *SchedularServer) awaitShutdown() error {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("Shutting down server...")

	return s.Stop()
}

func (s *SchedularServer) Stop() error {

	s.dbPool.Close()

	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(ctx)
	}

	log.Println("Scheduler server and database connection closed successfully")

	return nil
}

func (s *SchedularServer) insertTaskIntoDB(ctx context.Context, task Task) (string, error) {
	var taskId string
	err := s.dbPool.QueryRow(ctx, "INSERT INTO tasks (command, scheduled_at) VALUES ($1, $2) RETURNING id", task.Command, task.ScheduledAt).Scan(&taskId)
	if err != nil {
		return "", err
	}

	return taskId, nil
}
