# Product Requirements Document: Discovery API - Control Plane

**Version:** 1.0
**Date:** November 23, 2025
**Status:** Draft
**Owner:** AgentField Platform Team

---

## 1. Executive Summary

### 1.1 Purpose

The Discovery API enables developers and AI systems to dynamically discover and query available agent capabilities (reasoners and skills) from the AgentField control plane. This is critical for building intelligent AI orchestrators that can make just-in-time decisions about which agents, reasoners, or skills to invoke based on runtime requirements.

### 1.2 Problem Statement

Currently, developers and AI systems must:
- Manually track which agents are available and their capabilities
- Hardcode agent targets in their applications
- Have no programmatic way to discover reasoners/skills and their schemas
- Cannot build dynamic AI orchestrators that select tools at runtime

### 1.3 Success Criteria

- AI orchestrators can discover capabilities in <100ms (p95)
- Supports filtering by wildcards, tags, and agent IDs
- Returns schemas in multiple formats (JSON, XML, Compact)
- Scales to 1000+ concurrent discovery requests
- Zero-downtime deployments with caching

---

## 2. Goals & Non-Goals

### 2.1 Goals

✅ **Primary Goals:**
1. Provide a single, efficient REST endpoint for capability discovery
2. Support wildcard pattern matching for flexible filtering
3. Enable schema discovery for dynamic validation
4. Support multiple output formats for different use cases
5. Maintain high performance through intelligent caching
6. Provide clear, actionable error messages

✅ **Secondary Goals:**
1. Support filtering by multiple agent IDs simultaneously
2. Allow alias query parameters (`agent`/`node_id`, `agent_ids`/`node_ids`)
3. Enable tag-based filtering with wildcard support
4. Provide compact responses for bandwidth-constrained scenarios

### 2.2 Non-Goals

❌ **Out of Scope for v1:**
- Semantic/natural language search
- Performance metrics and analytics
- Cost estimation per capability
- Team-based filtering (no team concept yet)
- MCP server tool discovery
- WebSocket streaming for capability changes
- Authentication/authorization (handled at API gateway level)

---

## 3. API Specification

### 3.1 Endpoint

```
GET /api/v1/discovery/capabilities
```

**Purpose:** Discover available agent capabilities with flexible filtering and schema options.

### 3.2 Query Parameters

| Parameter | Type | Required | Description | Examples |
|-----------|------|----------|-------------|----------|
| `agent`<br>`node_id` | string | No | Filter by single agent ID (aliased) | `?agent=agent-001`<br>`?node_id=agent-001` |
| `agent_ids`<br>`node_ids` | string | No | Filter by multiple agent IDs (comma-separated, aliased) | `?agent_ids=agent-1,agent-2` |
| `reasoner` | string | No | Filter reasoners by pattern (supports wildcards) | `?reasoner=*research*`<br>`?reasoner=deep_*` |
| `skill` | string | No | Filter skills by pattern (supports wildcards) | `?skill=web_*` |
| `tags` | string | No | Filter by tags (comma-separated, supports wildcards) | `?tags=ml,nlp`<br>`?tags=ml*,*research` |
| `include_input_schema` | boolean | No | Include input schemas (default: false) | `?include_input_schema=true` |
| `include_output_schema` | boolean | No | Include output schemas (default: false) | `?include_output_schema=true` |
| `include_descriptions` | boolean | No | Include descriptions (default: true) | `?include_descriptions=false` |
| `include_examples` | boolean | No | Include usage examples (default: false) | `?include_examples=true` |
| `format` | string | No | Response format: `json`, `xml`, `compact` (default: json) | `?format=xml` |
| `health_status` | string | No | Filter by health status: `active`, `inactive`, `degraded` | `?health_status=active` |
| `limit` | integer | No | Maximum number of agents to return (default: 100, max: 500) | `?limit=50` |
| `offset` | integer | No | Pagination offset (default: 0) | `?offset=100` |

### 3.3 Wildcard Pattern Matching

**Valid Patterns:**
- `*abc*` - Contains "abc" anywhere
- `abc*` - Starts with "abc"
- `*abc` - Ends with "abc"
- `abc` - Exact match (no wildcards)

**Examples:**
- `?reasoner=*research*` → Matches: `deep_research`, `web_researcher`, `research_agent`
- `?tags=ml*` → Matches tags: `ml`, `mlops`, `ml_vision`
- `?skill=web_*` → Matches: `web_search`, `web_scraper`, `web_parser`

### 3.4 Response Format (JSON - Default)

```json
{
  "discovered_at": "2025-11-23T10:30:00Z",
  "total_agents": 5,
  "total_reasoners": 23,
  "total_skills": 45,
  "pagination": {
    "limit": 100,
    "offset": 0,
    "has_more": false
  },
  "capabilities": [
    {
      "agent_id": "agent-research-001",
      "base_url": "http://agent-research:8080",
      "version": "2.3.1",
      "health_status": "active",
      "deployment_type": "long_running",
      "last_heartbeat": "2025-11-23T10:29:45Z",

      "reasoners": [
        {
          "id": "deep_research",
          "description": "Performs comprehensive research using multiple sources and synthesizes findings",
          "tags": ["research", "ml", "synthesis"],

          "input_schema": {
            "type": "object",
            "properties": {
              "query": {
                "type": "string",
                "description": "Research query or topic"
              },
              "depth": {
                "type": "integer",
                "minimum": 1,
                "maximum": 5,
                "default": 3,
                "description": "Research depth level"
              },
              "sources": {
                "type": "array",
                "items": {"type": "string"},
                "description": "Specific sources to search"
              }
            },
            "required": ["query"]
          },

          "output_schema": {
            "type": "object",
            "properties": {
              "findings": {
                "type": "array",
                "items": {"type": "object"}
              },
              "confidence": {
                "type": "number",
                "minimum": 0,
                "maximum": 1
              },
              "citations": {
                "type": "array",
                "items": {"type": "string"}
              }
            }
          },

          "examples": [
            {
              "name": "Basic research query",
              "input": {
                "query": "Latest advances in quantum computing",
                "depth": 3
              },
              "description": "Performs mid-depth research on quantum computing"
            }
          ],

          "invocation_target": "agent-research-001:deep_research"
        }
      ],

      "skills": [
        {
          "id": "web_search",
          "description": "Search the web using multiple search engines",
          "tags": ["web", "search", "data"],

          "input_schema": {
            "type": "object",
            "properties": {
              "query": {"type": "string"},
              "num_results": {
                "type": "integer",
                "default": 10,
                "minimum": 1,
                "maximum": 100
              }
            },
            "required": ["query"]
          },

          "invocation_target": "agent-research-001:skill:web_search"
        }
      ]
    }
  ]
}
```

### 3.5 Response Format (XML)

When `?format=xml`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<discovery discovered_at="2025-11-23T10:30:00Z">
  <summary
    total_agents="5"
    total_reasoners="23"
    total_skills="45"/>

  <capabilities>
    <agent
      id="agent-research-001"
      base_url="http://agent-research:8080"
      version="2.3.1"
      health_status="active"
      deployment_type="long_running"
      last_heartbeat="2025-11-23T10:29:45Z">

      <reasoners>
        <reasoner
          id="deep_research"
          target="agent-research-001:deep_research">
          <description>Performs comprehensive research using multiple sources</description>
          <tags>
            <tag>research</tag>
            <tag>ml</tag>
            <tag>synthesis</tag>
          </tags>
          <input_schema>
            <field name="query" type="string" required="true">
              Research query or topic
            </field>
            <field name="depth" type="integer" min="1" max="5" default="3">
              Research depth level
            </field>
            <field name="sources" type="array">
              Specific sources to search
            </field>
          </input_schema>
          <output_schema>
            <field name="findings" type="array">Research findings</field>
            <field name="confidence" type="number" min="0" max="1">Confidence score</field>
          </output_schema>
        </reasoner>
      </reasoners>

      <skills>
        <skill
          id="web_search"
          target="agent-research-001:skill:web_search">
          <description>Search the web using multiple search engines</description>
          <tags>
            <tag>web</tag>
            <tag>search</tag>
          </tags>
        </skill>
      </skills>
    </agent>
  </capabilities>
</discovery>
```

### 3.6 Response Format (Compact)

When `?format=compact`:

```json
{
  "discovered_at": "2025-11-23T10:30:00Z",
  "reasoners": [
    {
      "id": "deep_research",
      "agent_id": "agent-research-001",
      "target": "agent-research-001:deep_research",
      "tags": ["research", "ml", "synthesis"]
    }
  ],
  "skills": [
    {
      "id": "web_search",
      "agent_id": "agent-research-001",
      "target": "agent-research-001:skill:web_search",
      "tags": ["web", "search", "data"]
    }
  ]
}
```

### 3.7 Error Responses

**400 Bad Request:**
```json
{
  "error": "invalid_parameter",
  "message": "Invalid format parameter. Must be one of: json, xml, compact",
  "details": {
    "parameter": "format",
    "provided": "yaml",
    "allowed": ["json", "xml", "compact"]
  }
}
```

**500 Internal Server Error:**
```json
{
  "error": "internal_error",
  "message": "Failed to retrieve agent capabilities",
  "request_id": "req_abc123"
}
```

---

## 4. Implementation Requirements

### 4.1 Performance Requirements

| Metric | Target | Measurement |
|--------|--------|-------------|
| Response Time (p50) | <50ms | Without schemas |
| Response Time (p95) | <100ms | Without schemas |
| Response Time (p99) | <200ms | With schemas |
| Throughput | 1000 req/s | Single instance |
| Cache Hit Rate | >95% | 30-second TTL |
| Memory Usage | <100MB | Per instance |

### 4.2 Scalability Requirements

1. **Horizontal Scaling:** Must support multiple control plane instances
2. **Caching Strategy:**
   - In-memory cache with 30-second TTL
   - Cache invalidation on agent registration/deregistration
   - Read-through cache pattern
3. **Database Queries:**
   - Maximum 1 database query per request (cached)
   - Use existing `ListAgents()` with no additional queries
   - All filtering done in-memory on cached data

### 4.3 Data Sources

**Primary:** `storage.StorageProvider.ListAgents()`
- Returns all active agents with their reasoners and skills
- Already includes health status and metadata
- No schema changes required

**Schema Requirements:**
- Reasoners must have: `ID`, `InputSchema`, `OutputSchema`, `Tags`
- Skills must have: `ID`, `InputSchema`, `Tags`
- All schema fields already exist in `types.AgentNode`

### 4.4 Handler Implementation (Go)

**File:** `control-plane/internal/handlers/discovery.go`

**Key Functions:**
1. `DiscoveryCapabilitiesHandler(storage.StorageProvider) gin.HandlerFunc`
2. `parseDiscoveryFilters(*gin.Context) DiscoveryFilters`
3. `buildDiscoveryResponse([]*types.AgentNode, DiscoveryFilters) DiscoveryResponse`
4. `matchesPattern(value string, pattern string) bool`
5. `matchesFilters(id string, tags []string, filters DiscoveryFilters) bool`
6. `formatJSONResponse(DiscoveryResponse) interface{}`
7. `formatXMLResponse(DiscoveryResponse) string`
8. `formatCompactResponse(DiscoveryResponse) CompactDiscoveryResponse`

**Caching Implementation:**
```go
var (
    agentCache      []*types.AgentNode
    agentCacheLock  sync.RWMutex
    agentCacheTime  time.Time
    agentCacheTTL   = 30 * time.Second
)

func getCachedAgents(ctx context.Context, storage storage.StorageProvider) ([]*types.AgentNode, error) {
    agentCacheLock.RLock()
    if time.Since(agentCacheTime) < agentCacheTTL && agentCache != nil {
        defer agentCacheLock.RUnlock()
        return agentCache, nil
    }
    agentCacheLock.RUnlock()

    agents, err := storage.ListAgents(ctx, types.AgentFilters{})
    if err != nil {
        return nil, err
    }

    agentCacheLock.Lock()
    agentCache = agents
    agentCacheTime = time.Now()
    agentCacheLock.Unlock()

    return agents, nil
}
```

### 4.5 Routing

**Add to:** `control-plane/internal/server/routes.go` (or equivalent)

```go
// Discovery API
discoveryGroup := api.Group("/discovery")
{
    discoveryGroup.GET("/capabilities", handlers.DiscoveryCapabilitiesHandler(storageProvider))
}
```

---

## 5. Testing Requirements

### 5.1 Unit Tests

**File:** `control-plane/internal/handlers/discovery_test.go`

**Test Cases:**
1. ✅ Basic discovery (no filters)
2. ✅ Filter by single agent_id
3. ✅ Filter by multiple agent_ids
4. ✅ Alias support (agent vs node_id)
5. ✅ Wildcard pattern matching (contains, prefix, suffix)
6. ✅ Tag filtering with wildcards
7. ✅ Reasoner filtering
8. ✅ Skill filtering
9. ✅ Schema inclusion flags
10. ✅ JSON format response
11. ✅ XML format response
12. ✅ Compact format response
13. ✅ Pagination (limit/offset)
14. ✅ Health status filtering
15. ✅ Cache hit/miss scenarios
16. ✅ Error handling (invalid format, invalid parameters)

### 5.2 Integration Tests

**File:** `tests/functional/tests/test_discovery_api.py`

**Scenarios:**
1. End-to-end discovery with real agent registration
2. Performance under load (1000 concurrent requests)
3. Cache invalidation on agent updates
4. Large result set pagination

### 5.3 Load Tests

**Tool:** `control-plane/tools/perf/discovery_load_test.go`

**Targets:**
- 1000 requests/second sustained
- p95 latency <100ms
- Cache hit rate >95%

---

## 6. Documentation Requirements

### 6.1 API Documentation

**File:** `docs/api/DISCOVERY_API.md`

**Sections:**
1. Overview and purpose
2. Authentication (if applicable)
3. Endpoint specification
4. Query parameter reference
5. Response format examples (all 3 formats)
6. Error codes and handling
7. Rate limiting (if applicable)
8. Best practices

### 6.2 Developer Guide

**File:** `docs/guides/USING_DISCOVERY_API.md`

**Topics:**
1. Common use cases
2. Example: AI orchestrator pattern
3. Example: Dynamic routing
4. Example: Schema validation
5. Performance optimization tips
6. Troubleshooting

---

## 7. Monitoring & Observability

### 7.1 Metrics

**Prometheus Metrics to Add:**
```
# Request metrics
agentfield_discovery_requests_total{format="json|xml|compact", status="success|error"}
agentfield_discovery_request_duration_seconds{format="json|xml|compact"}

# Cache metrics
agentfield_discovery_cache_hits_total
agentfield_discovery_cache_misses_total
agentfield_discovery_cache_size_bytes

# Filter usage metrics
agentfield_discovery_filter_usage{filter_type="reasoner|skill|tag|agent"}
```

### 7.2 Logging

**Log Events:**
- Discovery request received (DEBUG)
- Cache hit/miss (DEBUG)
- Filter application (DEBUG)
- Response sent (INFO with timing)
- Errors (ERROR with context)

**Log Format:**
```json
{
  "level": "info",
  "timestamp": "2025-11-23T10:30:00Z",
  "message": "discovery request completed",
  "request_id": "req_abc123",
  "filters": {
    "agent_ids": ["agent-1", "agent-2"],
    "reasoner": "*research*",
    "format": "json"
  },
  "results": {
    "agents": 2,
    "reasoners": 5,
    "skills": 10
  },
  "duration_ms": 45,
  "cache_hit": true
}
```

---

## 8. Migration & Rollout Plan

### 8.1 Phase 1: Development (Week 1)
- Implement handler and filtering logic
- Add unit tests
- Add integration tests
- Documentation

### 8.2 Phase 2: Internal Testing (Week 2)
- Deploy to staging environment
- Performance testing
- Load testing
- Bug fixes

### 8.3 Phase 3: Beta Release (Week 3)
- Deploy to production (behind feature flag)
- Beta users test integration
- Monitor metrics and logs
- Collect feedback

### 8.4 Phase 4: General Availability (Week 4)
- Remove feature flag
- Announce via documentation
- Update SDK examples
- Monitor adoption

---

## 9. Dependencies

### 9.1 Internal Dependencies
- ✅ `storage.StorageProvider.ListAgents()` - Already exists
- ✅ `types.AgentNode` schema - Already exists
- ✅ Gin router framework - Already in use

### 9.2 External Dependencies
- None

---

## 10. Open Questions

1. **Q:** Should we support regex patterns in addition to wildcards?
   **A:** No, wildcards are sufficient for v1. Regex adds complexity.

2. **Q:** Should filter combinations use AND or OR logic?
   **A:** AND logic (all filters must match). OR can be added later if needed.

3. **Q:** Should we support sorting (e.g., by agent_id, version)?
   **A:** Not in v1. Results can be sorted client-side.

4. **Q:** Should we expose internal agent metadata (last_heartbeat, health_score)?
   **A:** Yes, but only health_status and last_heartbeat for now.

---

## 11. Success Metrics

### 11.1 Adoption Metrics
- Number of unique clients using discovery API
- Discovery API requests per day
- Percentage of executions preceded by discovery call

### 11.2 Performance Metrics
- Average response time
- Cache hit rate
- Error rate

### 11.3 Business Metrics
- Number of AI orchestrator implementations built
- Developer satisfaction (via surveys)
- Time to integrate (target: <1 hour)

---

## 12. Appendix

### 12.1 Example Use Cases

**Use Case 1: AI Orchestrator**
```python
# AI discovers and selects appropriate reasoner
capabilities = app.discover(
    tags=["research"],
    include_input_schema=True
)

# AI selects best match and executes
selected = ai_select_best_reasoner(capabilities, user_query)
result = app.execute(target=selected.invocation_target, input=user_input)
```

**Use Case 2: Dynamic Tool Registry**
```python
# Build tool registry for LLM
tools = app.discover(
    format="compact",
    include_descriptions=True
)

# Pass to LLM for tool use
llm_response = openai.chat.completions.create(
    model="gpt-4",
    messages=[...],
    tools=convert_to_openai_tools(tools)
)
```

**Use Case 3: Health Monitoring**
```python
# Monitor available capabilities
healthy_agents = app.discover(
    health_status="active",
    include_input_schema=False
)

alert_if_below_threshold(len(healthy_agents.capabilities), min_agents=3)
```

### 12.2 References
- AgentField Architecture: `docs/ARCHITECTURE.md`
- Agent Registration API: `docs/api/AGENT_REGISTRATION.md`
- Execution API: `docs/api/EXECUTION_API.md`

---

**END OF DOCUMENT**
