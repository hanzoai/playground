package zap

import (
	"encoding/json"
	"testing"
)

func TestRequestIDMarshalInt(t *testing.T) {
	id := IntRequestID(42)
	b, err := json.Marshal(id)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "42" {
		t.Errorf("expected 42, got %s", b)
	}
}

func TestRequestIDUnmarshalString(t *testing.T) {
	var id RequestID
	if err := json.Unmarshal([]byte(`"abc"`), &id); err != nil {
		t.Fatal(err)
	}
	if id.Str == nil || *id.Str != "abc" {
		t.Errorf("expected string abc, got %+v", id)
	}
}

func TestRequestIDUnmarshalInt(t *testing.T) {
	var id RequestID
	if err := json.Unmarshal([]byte(`7`), &id); err != nil {
		t.Fatal(err)
	}
	if id.Int == nil || *id.Int != 7 {
		t.Errorf("expected int 7, got %+v", id)
	}
}

func TestSubmissionRoundTrip(t *testing.T) {
	sub := Submission{
		ID: "sub-1",
		Op: Op{Type: OpUserInput, Items: []UserInput{NewTextInput("hello")}},
	}

	b, err := json.Marshal(sub)
	if err != nil {
		t.Fatal(err)
	}

	var decoded Submission
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.ID != "sub-1" {
		t.Errorf("expected id sub-1, got %s", decoded.ID)
	}
	if decoded.Op.Type != OpUserInput {
		t.Errorf("expected op type %s, got %s", OpUserInput, decoded.Op.Type)
	}
	if len(decoded.Op.Items) != 1 || decoded.Op.Items[0].Text != "hello" {
		t.Errorf("unexpected items: %+v", decoded.Op.Items)
	}
}

func TestOpInterruptJSON(t *testing.T) {
	op := NewInterruptOp()
	b, err := json.Marshal(op)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["type"] != "interrupt" {
		t.Errorf("expected type interrupt, got %v", m["type"])
	}
}

func TestEventMsgUnmarshal(t *testing.T) {
	data := `{"type":"task_started","turn_id":"t-1","model_context_window":128000}`
	var evt EventMsg
	if err := json.Unmarshal([]byte(data), &evt); err != nil {
		t.Fatal(err)
	}
	if evt.Type != EventTurnStarted {
		t.Errorf("expected type %s, got %s", EventTurnStarted, evt.Type)
	}
	if len(evt.Raw) == 0 {
		t.Error("expected raw bytes to be preserved")
	}

	// Verify we can further decode the raw bytes into a typed event.
	var started TurnStartedEvent
	if err := json.Unmarshal(evt.Raw, &started); err != nil {
		t.Fatal(err)
	}
	if started.TurnID != "t-1" {
		t.Errorf("expected turn_id t-1, got %s", started.TurnID)
	}
	if started.ModelContextWindow == nil || *started.ModelContextWindow != 128000 {
		t.Errorf("expected model_context_window 128000, got %v", started.ModelContextWindow)
	}
}

func TestAgentMessageDeltaUnmarshal(t *testing.T) {
	data := `{"type":"agent_message_delta","delta":"Hello, "}`
	var evt EventMsg
	if err := json.Unmarshal([]byte(data), &evt); err != nil {
		t.Fatal(err)
	}
	if evt.Type != EventAgentMessageDelta {
		t.Errorf("expected type %s, got %s", EventAgentMessageDelta, evt.Type)
	}

	var delta AgentMessageDeltaEvent
	if err := json.Unmarshal(evt.Raw, &delta); err != nil {
		t.Fatal(err)
	}
	if delta.Delta != "Hello, " {
		t.Errorf("expected delta 'Hello, ', got %q", delta.Delta)
	}
}

func TestSandboxPolicyMarshal(t *testing.T) {
	sp := NewWorkspaceWriteSandbox()
	b, err := json.Marshal(sp)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["type"] != SandboxWorkspaceWrite {
		t.Errorf("expected type %s, got %v", SandboxWorkspaceWrite, m["type"])
	}
}

func TestInitializeParamsMarshal(t *testing.T) {
	params := InitializeParams{
		ClientInfo: ClientInfo{
			Name:    "playground",
			Version: "0.1.0",
		},
	}
	b, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}

	ci, ok := m["clientInfo"].(map[string]interface{})
	if !ok {
		t.Fatal("expected clientInfo object")
	}
	if ci["name"] != "playground" {
		t.Errorf("expected name playground, got %v", ci["name"])
	}
	if ci["version"] != "0.1.0" {
		t.Errorf("expected version 0.1.0, got %v", ci["version"])
	}
}

func TestJSONRPCMessageIsRequest(t *testing.T) {
	data := `{"id":1,"method":"initialize","params":{"clientInfo":{"name":"test","version":"0.1"}}}`
	var msg JSONRPCMessage
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		t.Fatal(err)
	}
	if !msg.IsRequest() {
		t.Error("expected message to be identified as request")
	}
	if msg.IsNotification() {
		t.Error("expected message NOT to be a notification")
	}
}

func TestJSONRPCMessageIsNotification(t *testing.T) {
	data := `{"method":"initialized"}`
	var msg JSONRPCMessage
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		t.Fatal(err)
	}
	if !msg.IsNotification() {
		t.Error("expected message to be a notification")
	}
	if msg.IsRequest() {
		t.Error("expected message NOT to be a request")
	}
}

func TestJSONRPCMessageIsResponse(t *testing.T) {
	data := `{"id":1,"result":{"userAgent":"codex/1.0","platformFamily":"unix","platformOs":"macos"}}`
	var msg JSONRPCMessage
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		t.Fatal(err)
	}
	if !msg.IsResponse() {
		t.Error("expected message to be a response")
	}
}

func TestJSONRPCMessageIsError(t *testing.T) {
	data := `{"id":1,"error":{"code":-32600,"message":"invalid request"}}`
	var msg JSONRPCMessage
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		t.Fatal(err)
	}
	if !msg.IsError() {
		t.Error("expected message to be an error")
	}
}

func TestExecCommandEndEventUnmarshal(t *testing.T) {
	data := `{
		"type":"exec_command_end",
		"call_id":"call-1",
		"turn_id":"turn-1",
		"command":["echo","hi"],
		"cwd":"/tmp",
		"stdout":"hi\n",
		"stderr":"",
		"aggregated_output":"hi\n",
		"exit_code":0,
		"duration":"1.234s",
		"formatted_output":"hi\n",
		"status":"completed"
	}`

	var evt EventMsg
	if err := json.Unmarshal([]byte(data), &evt); err != nil {
		t.Fatal(err)
	}
	if evt.Type != EventExecCommandEnd {
		t.Errorf("expected type %s, got %s", EventExecCommandEnd, evt.Type)
	}

	var end ExecCommandEndEvent
	if err := json.Unmarshal(evt.Raw, &end); err != nil {
		t.Fatal(err)
	}
	if end.ExitCode != 0 {
		t.Errorf("expected exit_code 0, got %d", end.ExitCode)
	}
	if end.Stdout != "hi\n" {
		t.Errorf("unexpected stdout: %q", end.Stdout)
	}
}

func TestMcpToolCallBeginEventUnmarshal(t *testing.T) {
	data := `{
		"type":"mcp_tool_call_begin",
		"call_id":"mcp-1",
		"invocation":{"server":"codex_apps","tool":"read_file","arguments":{"path":"/etc/hosts"}}
	}`

	var evt EventMsg
	if err := json.Unmarshal([]byte(data), &evt); err != nil {
		t.Fatal(err)
	}

	var begin McpToolCallBeginEvent
	if err := json.Unmarshal(evt.Raw, &begin); err != nil {
		t.Fatal(err)
	}
	if begin.Invocation.Server != "codex_apps" {
		t.Errorf("expected server codex_apps, got %s", begin.Invocation.Server)
	}
	if begin.Invocation.Tool != "read_file" {
		t.Errorf("expected tool read_file, got %s", begin.Invocation.Tool)
	}
}

func TestPoolBasicOps(t *testing.T) {
	p := NewPool()
	if p.Len() != 0 {
		t.Errorf("expected empty pool, got %d", p.Len())
	}

	_, ok := p.Get("bot-1")
	if ok {
		t.Error("expected Get to return false for missing bot")
	}

	result := p.ForSpace("space-1")
	if len(result) != 0 {
		t.Errorf("expected empty ForSpace result, got %d", len(result))
	}

	err := p.Remove("bot-1")
	if err == nil {
		t.Error("expected error removing non-existent bot")
	}
}
