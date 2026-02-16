// Package server provides the ZAP admin node for the playground control-plane.
//
// Replaces the old gRPC admin service with a ZAP-native node that exposes
// admin operations (list bots, health) via Cap'n Proto zero-copy messaging.
// Also exposes the same operations as REST endpoints on the existing Gin router.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/logger"
	"github.com/hanzoai/playground/control-plane/internal/storage"
	"github.com/hanzoai/playground/control-plane/pkg/types"
	"github.com/luxfi/zap"
)

const MsgTypeAdmin uint16 = 303

const (
	fieldPath   = 4
	fieldBody   = 12
	respStatus  = 0
	respBody    = 4
	respHeaders = 8
)

// zapAdminNode wraps a ZAP node for admin operations.
type zapAdminNode struct {
	node    *zap.Node
	storage storage.StorageProvider
}

// startZAPAdminNode creates and starts a ZAP admin node on the given port.
func startZAPAdminNode(port int, store storage.StorageProvider) (*zapAdminNode, error) {
	admin := &zapAdminNode{storage: store}

	node := zap.NewNode(zap.NodeConfig{
		NodeID:      "playground-admin",
		ServiceType: "_hanzo._tcp",
		Port:        port,
		Logger:      logger.SlogAdapter(),
	})

	node.Handle(MsgTypeAdmin, func(_ context.Context, _ string, msg *zap.Message) (*zap.Message, error) {
		return admin.handle(msg), nil
	})

	if err := node.Start(); err != nil {
		return nil, fmt.Errorf("zap admin: node start failed: %w", err)
	}

	admin.node = node
	logger.Logger.Info().Int("port", port).Msg("ZAP admin node listening")
	return admin, nil
}

func (a *zapAdminNode) stop() {
	if a.node != nil {
		a.node.Stop()
	}
}

func (a *zapAdminNode) handle(msg *zap.Message) *zap.Message {
	root := msg.Root()
	path := root.Text(fieldPath)

	switch path {
	case "/list-bots":
		return a.listBots()
	case "/health":
		return zapRespond(http.StatusOK, map[string]string{"status": "ok", "service": "playground-admin"})
	default:
		return zapRespond(http.StatusNotFound, map[string]string{"error": "unknown path: " + path})
	}
}

func (a *zapAdminNode) listBots() *zap.Message {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	nodes, err := a.storage.ListNodes(ctx, types.BotFilters{})
	if err != nil {
		return zapRespond(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	type botInfo struct {
		BotID  string `json:"bot_id"`
		NodeID string `json:"node_id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Status      string `json:"status"`
		NodeVersion string `json:"node_version"`
		LastHB      string `json:"last_heartbeat"`
	}

	var bots []botInfo
	for _, node := range nodes {
		if node == nil {
			continue
		}
		for _, r := range node.Bots {
			bots = append(bots, botInfo{
				BotID:  fmt.Sprintf("%s.%s", node.ID, r.ID),
				NodeID: node.ID,
				Name:        r.ID,
				Description: fmt.Sprintf("Bot %s from node %s", r.ID, node.ID),
				Status:      string(node.HealthStatus),
				NodeVersion: node.Version,
				LastHB:      node.LastHeartbeat.Format(time.RFC3339),
			})
		}
	}

	return zapRespond(http.StatusOK, map[string]interface{}{
		"bots": bots,
		"count":     len(bots),
	})
}

// zapRespond builds a ZAP response message.
func zapRespond(status int, data interface{}) *zap.Message {
	b := zap.NewBuilder(4096)
	ob := b.StartObject(12)
	ob.SetUint32(respStatus, uint32(status))
	body, _ := json.Marshal(data)
	ob.SetBytes(respBody, body)
	ob.SetBytes(respHeaders, []byte(`{"Content-Type":["application/json"]}`))
	ob.FinishAsRoot()
	msg, _ := zap.Parse(b.Finish())
	return msg
}

// registerAdminRESTRoutes adds REST endpoints for admin operations on the Gin router.
func registerAdminRESTRoutes(router *gin.Engine, store storage.StorageProvider) {
	admin := router.Group("/api/v1/admin")
	{
		admin.GET("/bots", func(c *gin.Context) {
			ctx := c.Request.Context()
			nodes, err := store.ListNodes(ctx, types.BotFilters{})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			type botInfo struct {
				BotID  string `json:"bot_id"`
				NodeID string `json:"node_id"`
				Name        string `json:"name"`
				Description string `json:"description"`
				Status      string `json:"status"`
				NodeVersion string `json:"node_version"`
				LastHB      string `json:"last_heartbeat"`
			}

			var bots []botInfo
			for _, node := range nodes {
				if node == nil {
					continue
				}
				for _, r := range node.Bots {
					bots = append(bots, botInfo{
						BotID:  fmt.Sprintf("%s.%s", node.ID, r.ID),
						NodeID: node.ID,
						Name:        r.ID,
						Description: fmt.Sprintf("Bot %s from node %s", r.ID, node.ID),
						Status:      string(node.HealthStatus),
						NodeVersion: node.Version,
						LastHB:      node.LastHeartbeat.Format(time.RFC3339),
					})
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"bots": bots,
				"count":     len(bots),
			})
		})
	}
}
