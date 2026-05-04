// Package zap provides Go types mirroring the hanzo/dev protocol and a
// WebSocket JSON-RPC client for communicating with dev sidecar processes.
//
// The types here are derived from:
//   - codex-rs/protocol/src/protocol.rs          (Submission, Op, EventMsg)
//   - codex-rs/app-server-protocol/src/lib.rs     (ClientRequest, ServerNotification)
//   - codex-rs/app-server-protocol/src/jsonrpc_lite.rs (JSONRPCMessage)
package zap

import "encoding/json"

// ---------------------------------------------------------------------------
// JSON-RPC wire types
// ---------------------------------------------------------------------------

// RequestID can be a string or integer per JSON-RPC 2.0.
type RequestID struct {
	Str *string
	Int *int64
}

func (r RequestID) MarshalJSON() ([]byte, error) {
	if r.Str != nil {
		return json.Marshal(*r.Str)
	}
	if r.Int != nil {
		return json.Marshal(*r.Int)
	}
	return json.Marshal(nil)
}

func (r *RequestID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		r.Str = &s
		return nil
	}
	var i int64
	if err := json.Unmarshal(data, &i); err == nil {
		r.Int = &i
		return nil
	}
	return nil
}

// IntRequestID is a convenience constructor.
func IntRequestID(id int64) RequestID {
	return RequestID{Int: &id}
}

// JSONRPCRequest is a request that expects a response.
type JSONRPCRequest struct {
	ID     RequestID        `json:"id"`
	Method string           `json:"method"`
	Params *json.RawMessage `json:"params,omitempty"`
}

// JSONRPCNotification is a notification with no response expected.
type JSONRPCNotification struct {
	Method string           `json:"method"`
	Params *json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse is a successful response to a request.
type JSONRPCResponse struct {
	ID     RequestID        `json:"id"`
	Result *json.RawMessage `json:"result,omitempty"`
}

// JSONRPCError is an error response.
type JSONRPCError struct {
	ID    RequestID          `json:"id"`
	Error JSONRPCErrorDetail `json:"error"`
}

// JSONRPCErrorDetail carries the error code, message, and optional data.
type JSONRPCErrorDetail struct {
	Code    int64            `json:"code"`
	Message string           `json:"message"`
	Data    *json.RawMessage `json:"data,omitempty"`
}

// JSONRPCMessage is the union of all JSON-RPC message types on the wire.
// We decode into this struct and inspect which fields are set.
type JSONRPCMessage struct {
	// Common
	Method string           `json:"method,omitempty"`
	Params *json.RawMessage `json:"params,omitempty"`

	// Request / Response
	ID *json.RawMessage `json:"id,omitempty"`

	// Response
	Result *json.RawMessage `json:"result,omitempty"`

	// Error
	Error *JSONRPCErrorDetail `json:"error,omitempty"`
}

// IsRequest returns true when the message has an id and a method (request).
func (m *JSONRPCMessage) IsRequest() bool {
	return m.ID != nil && m.Method != ""
}

// IsNotification returns true when the message has a method but no id.
func (m *JSONRPCMessage) IsNotification() bool {
	return m.ID == nil && m.Method != ""
}

// IsResponse returns true when the message has an id and a result.
func (m *JSONRPCMessage) IsResponse() bool {
	return m.ID != nil && m.Result != nil
}

// IsError returns true when the message has an error field.
func (m *JSONRPCMessage) IsError() bool {
	return m.Error != nil
}

// ---------------------------------------------------------------------------
// W3C Trace Context
// ---------------------------------------------------------------------------

type W3cTraceContext struct {
	Traceparent *string `json:"traceparent,omitempty"`
	Tracestate  *string `json:"tracestate,omitempty"`
}

// ---------------------------------------------------------------------------
// Submission Queue (client -> agent)
// ---------------------------------------------------------------------------

// Submission is the top-level envelope for operations sent to the agent.
type Submission struct {
	ID    string           `json:"id"`
	Op    Op               `json:"op"`
	Trace *W3cTraceContext `json:"trace,omitempty"`
}

// Op is the tagged union of all submission operations.
// The wire format uses "type" as the discriminator with snake_case values.
type Op struct {
	Type string `json:"type"`

	// UserInput fields
	Items                 []UserInput      `json:"items,omitempty"`
	FinalOutputJSONSchema *json.RawMessage `json:"final_output_json_schema,omitempty"`

	// UserTurn fields (superset of UserInput)
	Cwd               string           `json:"cwd,omitempty"`
	ApprovalPolicy    *AskForApproval  `json:"approval_policy,omitempty"`
	SandboxPolicy     *SandboxPolicy   `json:"sandbox_policy,omitempty"`
	Model             string           `json:"model,omitempty"`
	Effort            *string          `json:"effort,omitempty"`
	Summary           *string          `json:"summary,omitempty"`
	CollaborationMode *string          `json:"collaboration_mode,omitempty"`
	Personality       *string          `json:"personality,omitempty"`
	ServiceTier       *json.RawMessage `json:"service_tier,omitempty"`

	// ExecApproval / PatchApproval
	ApprovalID string          `json:"id,omitempty"`
	TurnID     string          `json:"turn_id,omitempty"`
	Decision   *ReviewDecision `json:"decision,omitempty"`

	// RealtimeConversationStart
	Prompt    string `json:"prompt,omitempty"`
	SessionID string `json:"session_id,omitempty"`

	// RealtimeConversationAudio
	Frame *RealtimeAudioFrame `json:"frame,omitempty"`

	// RealtimeConversationText
	Text string `json:"text,omitempty"`

	// SetThreadName
	Name string `json:"name,omitempty"`

	// ThreadRollback
	NumTurns *uint32 `json:"num_turns,omitempty"`

	// RunUserShellCommand
	Command string `json:"command,omitempty"`
}

// Op type constants matching the Rust serde tag values.
const (
	OpInterrupt                    = "interrupt"
	OpCleanBackgroundTerminals     = "clean_background_terminals"
	OpRealtimeConversationStart    = "realtime_conversation_start"
	OpRealtimeConversationAudio    = "realtime_conversation_audio"
	OpRealtimeConversationText     = "realtime_conversation_text"
	OpRealtimeConversationClose    = "realtime_conversation_close"
	OpUserInput                    = "user_input"
	OpUserTurn                     = "user_turn"
	OpOverrideTurnContext          = "override_turn_context"
	OpExecApproval                 = "exec_approval"
	OpPatchApproval                = "patch_approval"
	OpCompact                      = "compact"
	OpUndo                         = "undo"
	OpShutdown                     = "shutdown"
	OpListMcpTools                 = "list_mcp_tools"
	OpListModels                   = "list_models"
	OpSetThreadName                = "set_thread_name"
	OpThreadRollback               = "thread_rollback"
	OpRunUserShellCommand          = "run_user_shell_command"
)

// NewInterruptOp returns an Op for interrupting the current turn.
func NewInterruptOp() Op {
	return Op{Type: OpInterrupt}
}

// NewUserInputOp returns an Op for legacy user input.
func NewUserInputOp(items []UserInput) Op {
	return Op{Type: OpUserInput, Items: items}
}

// NewUserTurnOp returns an Op for a full user turn with context.
func NewUserTurnOp(items []UserInput, cwd, model string, approval AskForApproval, sandbox SandboxPolicy) Op {
	return Op{
		Type:           OpUserTurn,
		Items:          items,
		Cwd:            cwd,
		Model:          model,
		ApprovalPolicy: &approval,
		SandboxPolicy:  &sandbox,
	}
}

// NewShutdownOp returns an Op requesting agent shutdown.
func NewShutdownOp() Op {
	return Op{Type: OpShutdown}
}

// ---------------------------------------------------------------------------
// UserInput
// ---------------------------------------------------------------------------

// UserInput is a tagged union of user input types.
// Discriminated by Type. Only fields relevant to the active variant are set.
type UserInput struct {
	Type string `json:"type"`

	// Text variant
	Text         string        `json:"text,omitempty"`
	TextElements []TextElement `json:"text_elements,omitempty"`

	// Image variant
	ImageURL string `json:"image_url,omitempty"`

	// LocalImage / Skill variant (both use "path")
	Path string `json:"path,omitempty"`

	// Skill / Mention variant
	Name string `json:"name,omitempty"`
}

const (
	UserInputText       = "text"
	UserInputImage      = "image"
	UserInputLocalImage = "local_image"
	UserInputSkill      = "skill"
	UserInputMention    = "mention"
)

// NewTextInput creates a text user input.
func NewTextInput(text string) UserInput {
	return UserInput{Type: UserInputText, Text: text}
}

// TextElement marks a span within user text.
type TextElement struct {
	ByteRange   ByteRange `json:"byte_range"`
	Placeholder *string   `json:"placeholder,omitempty"`
}

// ByteRange is a half-open byte range [Start, End).
type ByteRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// ---------------------------------------------------------------------------
// RealtimeAudioFrame
// ---------------------------------------------------------------------------

// RealtimeAudioFrame carries base64-encoded audio data.
type RealtimeAudioFrame struct {
	Data              string  `json:"data"`
	SampleRate        uint32  `json:"sample_rate"`
	NumChannels       uint16  `json:"num_channels"`
	SamplesPerChannel *uint32 `json:"samples_per_channel,omitempty"`
	ItemID            *string `json:"item_id,omitempty"`
}

// ---------------------------------------------------------------------------
// Approval / Sandbox policies
// ---------------------------------------------------------------------------

// AskForApproval determines when the user is consulted for command approval.
// Wire values: "untrusted", "on-failure", "on-request", "granular", "never"
type AskForApproval string

const (
	ApprovalUnlessTrusted AskForApproval = "untrusted"
	ApprovalOnFailure     AskForApproval = "on-failure"
	ApprovalOnRequest     AskForApproval = "on-request"
	ApprovalNever         AskForApproval = "never"
)

// ReviewDecision is the user's approval response.
type ReviewDecision string

const (
	ReviewApprove ReviewDecision = "approve"
	ReviewDeny    ReviewDecision = "deny"
	ReviewAlways  ReviewDecision = "always_approve"
)

// SandboxPolicy determines execution restrictions.
// Uses "type" discriminator on the wire.
type SandboxPolicy struct {
	Type string `json:"type"`

	// WorkspaceWrite fields
	WritableRoots      []string `json:"writable_roots,omitempty"`
	NetworkAccess      *bool    `json:"network_access,omitempty"`
	ExcludeTmpdirEnv   *bool    `json:"exclude_tmpdir_env_var,omitempty"`
	ExcludeSlashTmp    *bool    `json:"exclude_slash_tmp,omitempty"`
}

const (
	SandboxDangerFullAccess = "danger-full-access"
	SandboxReadOnly         = "read-only"
	SandboxExternalSandbox  = "external-sandbox"
	SandboxWorkspaceWrite   = "workspace-write"
)

// NewWorkspaceWriteSandbox returns a workspace-write sandbox with no network.
func NewWorkspaceWriteSandbox() SandboxPolicy {
	f := false
	return SandboxPolicy{
		Type:          SandboxWorkspaceWrite,
		NetworkAccess: &f,
	}
}

// ---------------------------------------------------------------------------
// Event Queue (agent -> client)
// ---------------------------------------------------------------------------

// EventMsg is the tagged union of all events from the agent.
// The wire format uses "type" as discriminator with snake_case values.
type EventMsg struct {
	Type string `json:"type"`

	// Raw payload for consumers that need the full event.
	Raw json.RawMessage `json:"-"`
}

// UnmarshalJSON decodes the event, preserving the raw bytes.
func (e *EventMsg) UnmarshalJSON(data []byte) error {
	e.Raw = append(e.Raw[:0], data...)
	type alias struct {
		Type string `json:"type"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	e.Type = a.Type
	return nil
}

// Event type constants matching the Rust serde tag values.
const (
	EventError                          = "error"
	EventWarning                        = "warning"
	EventRealtimeConversationStarted    = "realtime_conversation_started"
	EventRealtimeConversationRealtime   = "realtime_conversation_realtime"
	EventRealtimeConversationClosed     = "realtime_conversation_closed"
	EventModelReroute                   = "model_reroute"
	EventContextCompacted               = "context_compacted"
	EventThreadRolledBack               = "thread_rolled_back"
	EventTurnStarted                    = "task_started" // v1 wire name
	EventTurnComplete                   = "task_complete" // v1 wire name
	EventTokenCount                     = "token_count"
	EventAgentMessage                   = "agent_message"
	EventUserMessage                    = "user_message"
	EventAgentMessageDelta              = "agent_message_delta"
	EventAgentReasoning                 = "agent_reasoning"
	EventAgentReasoningDelta            = "agent_reasoning_delta"
	EventAgentReasoningRawContent       = "agent_reasoning_raw_content"
	EventAgentReasoningRawContentDelta  = "agent_reasoning_raw_content_delta"
	EventAgentReasoningSectionBreak     = "agent_reasoning_section_break"
	EventSessionConfigured              = "session_configured"
	EventThreadNameUpdated              = "thread_name_updated"
	EventMcpStartupUpdate               = "mcp_startup_update"
	EventMcpStartupComplete             = "mcp_startup_complete"
	EventMcpToolCallBegin               = "mcp_tool_call_begin"
	EventMcpToolCallEnd                 = "mcp_tool_call_end"
	EventWebSearchBegin                 = "web_search_begin"
	EventWebSearchEnd                   = "web_search_end"
	EventImageGenerationBegin           = "image_generation_begin"
	EventImageGenerationEnd             = "image_generation_end"
	EventExecCommandBegin               = "exec_command_begin"
	EventExecCommandOutputDelta         = "exec_command_output_delta"
	EventTerminalInteraction            = "terminal_interaction"
	EventExecCommandEnd                 = "exec_command_end"
	EventViewImageToolCall              = "view_image_tool_call"
	EventExecApprovalRequest            = "exec_approval_request"
	EventRequestPermissions             = "request_permissions"
	EventRequestUserInput               = "request_user_input"
	EventDynamicToolCallRequest         = "dynamic_tool_call_request"
	EventDynamicToolCallResponse        = "dynamic_tool_call_response"
	EventElicitationRequest             = "elicitation_request"
	EventApplyPatchApprovalRequest      = "apply_patch_approval_request"
	EventGuardianAssessment             = "guardian_assessment"
	EventDeprecationNotice              = "deprecation_notice"
	EventBackgroundEvent                = "background_event"
	EventUndoStarted                    = "undo_started"
	EventUndoCompleted                  = "undo_completed"
	EventStreamError                    = "stream_error"
	EventPatchApplyBegin                = "patch_apply_begin"
	EventPatchApplyEnd                  = "patch_apply_end"
	EventTurnDiff                       = "turn_diff"
	EventGetHistoryEntryResponse        = "get_history_entry_response"
	EventMcpListToolsResponse           = "mcp_list_tools_response"
	EventListCustomPromptsResponse      = "list_custom_prompts_response"
	EventListSkillsResponse             = "list_skills_response"
	EventSkillsUpdateAvailable          = "skills_update_available"
	EventPlanUpdate                     = "plan_update"
	EventTurnAborted                    = "turn_aborted"
	EventShutdownComplete               = "shutdown_complete"
	EventEnteredReviewMode              = "entered_review_mode"
	EventExitedReviewMode               = "exited_review_mode"
	EventRawResponseItem                = "raw_response_item"
	EventItemStarted                    = "item_started"
	EventItemCompleted                  = "item_completed"
	EventHookStarted                    = "hook_started"
	EventHookCompleted                  = "hook_completed"
	EventAgentMessageContentDelta       = "agent_message_content_delta"
	EventPlanDelta                      = "plan_delta"
	EventReasoningContentDelta          = "reasoning_content_delta"
	EventReasoningRawContentDelta       = "reasoning_raw_content_delta"
	EventCollabAgentSpawnBegin          = "collab_agent_spawn_begin"
	EventCollabAgentSpawnEnd            = "collab_agent_spawn_end"
	EventCollabAgentInteractionBegin    = "collab_agent_interaction_begin"
	EventCollabAgentInteractionEnd      = "collab_agent_interaction_end"
	EventCollabWaitingBegin             = "collab_waiting_begin"
	EventCollabWaitingEnd               = "collab_waiting_end"
	EventCollabCloseBegin               = "collab_close_begin"
	EventCollabCloseEnd                 = "collab_close_end"
	EventCollabResumeBegin              = "collab_resume_begin"
	EventCollabResumeEnd                = "collab_resume_end"
)

// Typed event payloads for the most commonly consumed events.

// ErrorEvent carries an error message from the agent.
type ErrorEvent struct {
	Message       string  `json:"message"`
	CodexErrorInfo *string `json:"codex_error_info,omitempty"`
}

// TurnStartedEvent signals the agent has begun a turn.
type TurnStartedEvent struct {
	TurnID              string `json:"turn_id"`
	ModelContextWindow  *int64 `json:"model_context_window,omitempty"`
	CollaborationMode   string `json:"collaboration_mode_kind,omitempty"`
}

// TurnCompleteEvent signals the agent has finished a turn.
type TurnCompleteEvent struct {
	TurnID           string  `json:"turn_id"`
	LastAgentMessage *string `json:"last_agent_message,omitempty"`
}

// AgentMessageEvent carries a complete agent text message.
type AgentMessageEvent struct {
	Message string  `json:"message"`
	Phase   *string `json:"phase,omitempty"`
}

// AgentMessageDeltaEvent carries an incremental text delta.
type AgentMessageDeltaEvent struct {
	Delta string `json:"delta"`
}

// ExecCommandBeginEvent signals the start of a shell command execution.
type ExecCommandBeginEvent struct {
	CallID    string   `json:"call_id"`
	ProcessID *string  `json:"process_id,omitempty"`
	TurnID    string   `json:"turn_id"`
	Command   []string `json:"command"`
	Cwd       string   `json:"cwd"`
}

// ExecCommandEndEvent signals the end of a shell command execution.
type ExecCommandEndEvent struct {
	CallID          string   `json:"call_id"`
	ProcessID       *string  `json:"process_id,omitempty"`
	TurnID          string   `json:"turn_id"`
	Command         []string `json:"command"`
	Cwd             string   `json:"cwd"`
	Stdout          string   `json:"stdout"`
	Stderr          string   `json:"stderr"`
	AggregatedOutput string  `json:"aggregated_output"`
	ExitCode        int      `json:"exit_code"`
	Duration        string   `json:"duration"`
	FormattedOutput string   `json:"formatted_output"`
	Status          string   `json:"status"`
}

// McpToolCallBeginEvent signals the start of an MCP tool call.
type McpToolCallBeginEvent struct {
	CallID     string        `json:"call_id"`
	Invocation McpInvocation `json:"invocation"`
}

// McpToolCallEndEvent signals the end of an MCP tool call.
type McpToolCallEndEvent struct {
	CallID     string        `json:"call_id"`
	Invocation McpInvocation `json:"invocation"`
	Duration   string        `json:"duration"`
}

// McpInvocation identifies an MCP server tool call.
type McpInvocation struct {
	Server    string           `json:"server"`
	Tool      string           `json:"tool"`
	Arguments *json.RawMessage `json:"arguments,omitempty"`
}

// PatchApplyBeginEvent signals the start of a code patch application.
type PatchApplyBeginEvent struct {
	CallID string `json:"call_id"`
	TurnID string `json:"turn_id"`
}

// PatchApplyEndEvent signals the end of a code patch application.
type PatchApplyEndEvent struct {
	CallID string `json:"call_id"`
	TurnID string `json:"turn_id"`
	Status string `json:"status"`
}

// TokenUsage carries token consumption info.
type TokenUsage struct {
	InputTokens          int64 `json:"input_tokens"`
	CachedInputTokens    int64 `json:"cached_input_tokens"`
	OutputTokens         int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
	TotalTokens          int64 `json:"total_tokens"`
}

// TokenUsageInfo carries session and last-turn usage.
type TokenUsageInfo struct {
	TotalTokenUsage    TokenUsage `json:"total_token_usage"`
	LastTokenUsage     TokenUsage `json:"last_token_usage"`
	ModelContextWindow *int64     `json:"model_context_window,omitempty"`
}

// TokenCountEvent carries token usage for the session.
type TokenCountEvent struct {
	Info *TokenUsageInfo `json:"info,omitempty"`
}

// ---------------------------------------------------------------------------
// TurnItem (structured turn items used in item/started, item/completed)
// ---------------------------------------------------------------------------

// TurnItem is the tagged union of turn item types.
type TurnItem struct {
	Type string `json:"type"`

	// UserMessage
	ID      string      `json:"id,omitempty"`
	Content interface{} `json:"content,omitempty"`

	// AgentMessage
	Phase *string `json:"phase,omitempty"`

	// Plan
	PlanText string `json:"text,omitempty"`

	// Reasoning
	SummaryText []string `json:"summary_text,omitempty"`
	RawContent  []string `json:"raw_content,omitempty"`
}

// ---------------------------------------------------------------------------
// ContentItem / ResponseItem (model I/O)
// ---------------------------------------------------------------------------

// ContentItem is a tagged union of content pieces.
type ContentItem struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

const (
	ContentInputText  = "input_text"
	ContentInputImage = "input_image"
	ContentOutputText = "output_text"
)

// ResponseItem is a tagged union of model response items.
type ResponseItem struct {
	Type string `json:"type"`

	// Message
	Role    string        `json:"role,omitempty"`
	Content []ContentItem `json:"content,omitempty"`
	EndTurn *bool         `json:"end_turn,omitempty"`

	// LocalShellCall
	CallID string `json:"call_id,omitempty"`
	Status string `json:"status,omitempty"`

	// FunctionCall
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// ---------------------------------------------------------------------------
// Initialize params (app-server protocol)
// ---------------------------------------------------------------------------

// ClientInfo identifies the connecting client.
type ClientInfo struct {
	Name    string  `json:"name"`
	Title   *string `json:"title,omitempty"`
	Version string  `json:"version"`
}

// InitializeCapabilities declares client capabilities.
type InitializeCapabilities struct {
	ExperimentalAPI           bool     `json:"experimentalApi,omitempty"`
	OptOutNotificationMethods []string `json:"optOutNotificationMethods,omitempty"`
}

// InitializeParams is sent as the first request to the app-server.
type InitializeParams struct {
	ClientInfo   ClientInfo              `json:"clientInfo"`
	Capabilities *InitializeCapabilities `json:"capabilities,omitempty"`
}

// InitializeResponse is the server's reply to initialize.
type InitializeResponse struct {
	UserAgent      string `json:"userAgent"`
	PlatformFamily string `json:"platformFamily"`
	PlatformOS     string `json:"platformOs"`
}

// ---------------------------------------------------------------------------
// ConnectParams for sidecar spawning
// ---------------------------------------------------------------------------

// ConnectParams holds the information needed to connect to a running sidecar.
type ConnectParams struct {
	// WebSocket URL, e.g. "ws://127.0.0.1:PORT".
	WSURL string
}
