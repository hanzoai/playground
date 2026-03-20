package tasks

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

// PGStore is a PostgreSQL-backed task store. Thread-safe via DB transactions.
// Replaces the in-memory Store when the control plane runs in postgres mode.
type PGStore struct {
	db *sql.DB
}

// NewPGStore creates a task store backed by PostgreSQL.
func NewPGStore(db *sql.DB) *PGStore {
	return &PGStore{db: db}
}

func (s *PGStore) CreateTask(task *Task) error {
	if task.ID == "" {
		return ErrEmptyTaskID
	}
	if task.SpaceID == "" {
		return ErrEmptySpaceID
	}
	if task.Title == "" {
		return ErrEmptyTitle
	}

	now := time.Now().UTC()
	task.CreatedAt = now
	task.UpdatedAt = now
	if task.State == "" {
		task.State = TaskPending
	}

	dependsOn, _ := json.Marshal(task.DependsOn)
	labels, _ := json.Marshal(task.Labels)
	input, _ := json.Marshal(task.Input)
	output, _ := json.Marshal(task.Output)
	metadata, _ := json.Marshal(task.Metadata)

	_, err := s.db.Exec(`
		INSERT INTO tasks (id, space_id, title, description, state, priority,
			assigned_to, created_by, workflow_id, parent_task_id,
			depends_on, labels, input, output, error, progress,
			max_retries, retry_count, timeout, metadata, started_at, completed_at,
			created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24)`,
		task.ID, task.SpaceID, task.Title, task.Description, task.State, task.Priority,
		task.AssignedTo, task.CreatedBy, task.WorkflowID, task.ParentTaskID,
		dependsOn, labels, input, output, task.Error, task.Progress,
		task.MaxRetries, task.RetryCount, int64(task.Timeout), metadata,
		nilTime(task.StartedAt), nilTime(task.CompletedAt),
		task.CreatedAt, task.UpdatedAt,
	)
	return err
}

func (s *PGStore) GetTask(id string) (*Task, error) {
	row := s.db.QueryRow(`SELECT id, space_id, title, description, state, priority,
		assigned_to, created_by, workflow_id, parent_task_id,
		depends_on, labels, input, output, error, progress,
		max_retries, retry_count, timeout, metadata, started_at, completed_at,
		created_at, updated_at FROM tasks WHERE id = $1`, id)
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTaskNotFound
	}
	return t, err
}

func (s *PGStore) UpdateTask(task *Task) error {
	task.UpdatedAt = time.Now().UTC()
	labels, _ := json.Marshal(task.Labels)
	metadata, _ := json.Marshal(task.Metadata)
	res, err := s.db.Exec(`UPDATE tasks SET title=$1, description=$2, priority=$3,
		labels=$4, metadata=$5, updated_at=$6 WHERE id=$7`,
		task.Title, task.Description, task.Priority,
		labels, metadata, task.UpdatedAt, task.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrTaskNotFound
	}
	return nil
}

func (s *PGStore) DeleteTask(id string) error {
	res, err := s.db.Exec(`DELETE FROM tasks WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrTaskNotFound
	}
	return nil
}

func (s *PGStore) ListTasks(spaceID string, filters TaskFilters) []*Task {
	query := `SELECT id, space_id, title, description, state, priority,
		assigned_to, created_by, workflow_id, parent_task_id,
		depends_on, labels, input, output, error, progress,
		max_retries, retry_count, timeout, metadata, started_at, completed_at,
		created_at, updated_at FROM tasks WHERE space_id = $1`
	args := []any{spaceID}
	idx := 2

	if filters.State != nil {
		query += ` AND state = $` + itoa(idx)
		args = append(args, string(*filters.State))
		idx++
	}
	if filters.AssignedTo != nil {
		query += ` AND assigned_to = $` + itoa(idx)
		args = append(args, *filters.AssignedTo)
		idx++
	}
	if filters.Priority != nil {
		query += ` AND priority = $` + itoa(idx)
		args = append(args, int(*filters.Priority))
		idx++
	}
	if filters.WorkflowID != nil {
		query += ` AND workflow_id = $` + itoa(idx)
		args = append(args, *filters.WorkflowID)
		idx++
	}

	query += ` ORDER BY priority DESC, created_at ASC`

	if filters.Limit > 0 {
		query += ` LIMIT $` + itoa(idx)
		args = append(args, filters.Limit)
		idx++
	}
	if filters.Offset > 0 {
		query += ` OFFSET $` + itoa(idx)
		args = append(args, filters.Offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var result []*Task
	for rows.Next() {
		t, err := scanTaskRows(rows)
		if err != nil {
			continue
		}
		result = append(result, t)
	}
	return result
}

func (s *PGStore) ClaimTask(taskID, agentID string) error {
	res, err := s.db.Exec(`UPDATE tasks SET state=$1, assigned_to=$2, updated_at=$3
		WHERE id=$4 AND state='pending'`,
		TaskClaimed, agentID, time.Now().UTC(), taskID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// Check if task exists
		var state string
		err := s.db.QueryRow(`SELECT state FROM tasks WHERE id=$1`, taskID).Scan(&state)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTaskNotFound
		}
		return ErrAlreadyClaimed
	}
	return nil
}

func (s *PGStore) StartTask(taskID string) error {
	now := time.Now().UTC()
	res, err := s.db.Exec(`UPDATE tasks SET state=$1, started_at=$2, updated_at=$3
		WHERE id=$4 AND state IN ('pending','claimed')`,
		TaskRunning, now, now, taskID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrInvalidTransition
	}
	return nil
}

func (s *PGStore) CompleteTask(taskID string, output map[string]any) error {
	now := time.Now().UTC()
	outputJSON, _ := json.Marshal(output)
	res, err := s.db.Exec(`UPDATE tasks SET state=$1, output=$2, progress=100,
		completed_at=$3, updated_at=$4
		WHERE id=$5 AND state='running'`,
		TaskCompleted, outputJSON, now, now, taskID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrInvalidTransition
	}
	return nil
}

func (s *PGStore) FailTask(taskID string, errMsg string) error {
	now := time.Now().UTC()

	// Use a transaction for atomic read-modify-write.
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	var state string
	var retryCount, maxRetries int
	err = tx.QueryRow(`SELECT state, retry_count, max_retries FROM tasks WHERE id=$1 FOR UPDATE`, taskID).
		Scan(&state, &retryCount, &maxRetries)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTaskNotFound
	}
	if err != nil {
		return err
	}
	if state != string(TaskRunning) {
		return ErrInvalidTransition
	}

	if retryCount < maxRetries {
		_, err = tx.Exec(`UPDATE tasks SET state='pending', error=$1, retry_count=$2,
			assigned_to='', started_at=NULL, progress=0, updated_at=$3 WHERE id=$4`,
			errMsg, retryCount+1, now, taskID)
	} else {
		_, err = tx.Exec(`UPDATE tasks SET state='failed', error=$1, completed_at=$2, updated_at=$3 WHERE id=$4`,
			errMsg, now, now, taskID)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PGStore) CancelTask(taskID string) error {
	now := time.Now().UTC()
	res, err := s.db.Exec(`UPDATE tasks SET state=$1, completed_at=$2, updated_at=$3
		WHERE id=$4 AND state NOT IN ('completed','cancelled')`,
		TaskCancelled, now, now, taskID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrInvalidTransition
	}
	return nil
}

func (s *PGStore) UpdateProgress(taskID string, progress int) error {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	res, err := s.db.Exec(`UPDATE tasks SET progress=$1, updated_at=$2
		WHERE id=$3 AND state='running'`,
		progress, time.Now().UTC(), taskID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrInvalidTransition
	}
	return nil
}

func (s *PGStore) GetNextPendingTask(spaceID string, agentID string) (*Task, error) {
	now := time.Now().UTC()
	row := s.db.QueryRow(`UPDATE tasks SET state='claimed', assigned_to=$1, updated_at=$2
		WHERE id = (
			SELECT id FROM tasks
			WHERE space_id=$3 AND state='pending'
			AND (tasks.depends_on IS NULL OR tasks.depends_on = '[]'::jsonb OR NOT EXISTS (
				SELECT 1 FROM jsonb_array_elements_text(tasks.depends_on) dep_id
				JOIN tasks d ON d.id = dep_id
				WHERE d.state != 'completed'
			))
			ORDER BY priority DESC, created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, space_id, title, description, state, priority,
			assigned_to, created_by, workflow_id, parent_task_id,
			depends_on, labels, input, output, error, progress,
			max_retries, retry_count, timeout, metadata, started_at, completed_at,
			created_at, updated_at`,
		agentID, now, spaceID)

	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return t, err
}

func (s *PGStore) CreateWorkflow(wf *Workflow) error {
	if wf.ID == "" || wf.SpaceID == "" {
		return ErrEmptySpaceID
	}
	now := time.Now().UTC()
	wf.CreatedAt = now
	wf.UpdatedAt = now
	if wf.State == "" {
		wf.State = TaskPending
	}
	tasksJSON, _ := json.Marshal(wf.Tasks)
	metadata, _ := json.Marshal(wf.Metadata)
	_, err := s.db.Exec(`INSERT INTO task_workflows (id, space_id, name, description, state, tasks, created_by, metadata, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		wf.ID, wf.SpaceID, wf.Name, wf.Description, wf.State, tasksJSON, wf.CreatedBy, metadata, wf.CreatedAt, wf.UpdatedAt)
	return err
}

func (s *PGStore) GetWorkflow(id string) (*Workflow, error) {
	var wf Workflow
	var tasksJSON, metadataJSON []byte
	var completedAt sql.NullTime
	err := s.db.QueryRow(`SELECT id, space_id, name, description, state, tasks, created_by, metadata, completed_at, created_at, updated_at
		FROM task_workflows WHERE id=$1`, id).
		Scan(&wf.ID, &wf.SpaceID, &wf.Name, &wf.Description, &wf.State, &tasksJSON, &wf.CreatedBy, &metadataJSON, &completedAt, &wf.CreatedAt, &wf.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrWorkflowNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(tasksJSON, &wf.Tasks)
	_ = json.Unmarshal(metadataJSON, &wf.Metadata)
	if completedAt.Valid {
		wf.CompletedAt = &completedAt.Time
	}
	return &wf, nil
}

func (s *PGStore) ListWorkflows(spaceID string) []*Workflow {
	rows, err := s.db.Query(`SELECT id, space_id, name, description, state, tasks, created_by, metadata, completed_at, created_at, updated_at
		FROM task_workflows WHERE space_id=$1 ORDER BY created_at ASC`, spaceID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var result []*Workflow
	for rows.Next() {
		var wf Workflow
		var tasksJSON, metadataJSON []byte
		var completedAt sql.NullTime
		if err := rows.Scan(&wf.ID, &wf.SpaceID, &wf.Name, &wf.Description, &wf.State, &tasksJSON, &wf.CreatedBy, &metadataJSON, &completedAt, &wf.CreatedAt, &wf.UpdatedAt); err != nil {
			continue
		}
		_ = json.Unmarshal(tasksJSON, &wf.Tasks)
		_ = json.Unmarshal(metadataJSON, &wf.Metadata)
		if completedAt.Valid {
			wf.CompletedAt = &completedAt.Time
		}
		result = append(result, &wf)
	}
	return result
}

func (s *PGStore) UpdateWorkflow(wf *Workflow) error {
	wf.UpdatedAt = time.Now().UTC()
	tasksJSON, _ := json.Marshal(wf.Tasks)
	metadata, _ := json.Marshal(wf.Metadata)
	res, err := s.db.Exec(`UPDATE workflows SET name=$1, description=$2, state=$3, tasks=$4, metadata=$5, completed_at=$6, updated_at=$7 WHERE id=$8`,
		wf.Name, wf.Description, wf.State, tasksJSON, metadata, nilTime(wf.CompletedAt), wf.UpdatedAt, wf.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrWorkflowNotFound
	}
	return nil
}

func (s *PGStore) ListSpaceIDs() []string {
	rows, err := s.db.Query(`SELECT DISTINCT space_id FROM tasks`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

func (s *PGStore) ListActiveTasks() []*Task {
	rows, err := s.db.Query(`SELECT id, space_id, title, description, state, priority,
		assigned_to, created_by, workflow_id, parent_task_id,
		depends_on, labels, input, output, error, progress,
		max_retries, retry_count, timeout, metadata, started_at, completed_at,
		created_at, updated_at FROM tasks WHERE state IN ('pending','claimed','running','retrying')`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []*Task
	for rows.Next() {
		t, err := scanTaskRows(rows)
		if err != nil {
			continue
		}
		result = append(result, t)
	}
	return result
}

func (s *PGStore) ListActiveWorkflows() []*Workflow {
	rows, err := s.db.Query(`SELECT id, space_id, name, description, state, tasks, created_by, metadata, completed_at, created_at, updated_at
		FROM task_workflows WHERE state NOT IN ('completed','failed','cancelled')`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []*Workflow
	for rows.Next() {
		var wf Workflow
		var tasksJSON, metadataJSON []byte
		var completedAt sql.NullTime
		if err := rows.Scan(&wf.ID, &wf.SpaceID, &wf.Name, &wf.Description, &wf.State, &tasksJSON, &wf.CreatedBy, &metadataJSON, &completedAt, &wf.CreatedAt, &wf.UpdatedAt); err != nil {
			continue
		}
		_ = json.Unmarshal(tasksJSON, &wf.Tasks)
		_ = json.Unmarshal(metadataJSON, &wf.Metadata)
		if completedAt.Valid {
			wf.CompletedAt = &completedAt.Time
		}
		result = append(result, &wf)
	}
	return result
}

// --- helpers ---

func scanTask(row *sql.Row) (*Task, error) {
	var t Task
	var dependsOn, labels, input, output, metadata []byte
	var startedAt, completedAt sql.NullTime
	var timeoutNs int64
	err := row.Scan(&t.ID, &t.SpaceID, &t.Title, &t.Description, &t.State, &t.Priority,
		&t.AssignedTo, &t.CreatedBy, &t.WorkflowID, &t.ParentTaskID,
		&dependsOn, &labels, &input, &output, &t.Error, &t.Progress,
		&t.MaxRetries, &t.RetryCount, &timeoutNs, &metadata,
		&startedAt, &completedAt, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(dependsOn, &t.DependsOn)
	_ = json.Unmarshal(labels, &t.Labels)
	_ = json.Unmarshal(input, &t.Input)
	_ = json.Unmarshal(output, &t.Output)
	_ = json.Unmarshal(metadata, &t.Metadata)
	t.Timeout = time.Duration(timeoutNs)
	if startedAt.Valid {
		t.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		t.CompletedAt = &completedAt.Time
	}
	return &t, nil
}

func scanTaskRows(rows *sql.Rows) (*Task, error) {
	var t Task
	var dependsOn, labels, input, output, metadata []byte
	var startedAt, completedAt sql.NullTime
	var timeoutNs int64
	err := rows.Scan(&t.ID, &t.SpaceID, &t.Title, &t.Description, &t.State, &t.Priority,
		&t.AssignedTo, &t.CreatedBy, &t.WorkflowID, &t.ParentTaskID,
		&dependsOn, &labels, &input, &output, &t.Error, &t.Progress,
		&t.MaxRetries, &t.RetryCount, &timeoutNs, &metadata,
		&startedAt, &completedAt, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(dependsOn, &t.DependsOn)
	_ = json.Unmarshal(labels, &t.Labels)
	_ = json.Unmarshal(input, &t.Input)
	_ = json.Unmarshal(output, &t.Output)
	_ = json.Unmarshal(metadata, &t.Metadata)
	t.Timeout = time.Duration(timeoutNs)
	if startedAt.Valid {
		t.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		t.CompletedAt = &completedAt.Time
	}
	return &t, nil
}

func nilTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
