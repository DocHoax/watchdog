package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/topology"
)

// handleTopologyRoute multiplexes /api/v1/topology and /api/v1/topology/* requests.
func (s *Server) handleTopologyRoute(w http.ResponseWriter, r *http.Request) {
	topoSvc := s.TopologyService()
	if topoSvc == nil {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, "TOPOLOGY_SERVICE_UNAVAILABLE", "Topology service is not configured")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/topology")
	path = strings.TrimPrefix(path, "/")

	// 1. /api/v1/topology or /api/v1/topology/
	if path == "" {
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			s.handleGetTopology(w, r, topoSvc)
		default:
			w.Header().Set("Allow", "GET, HEAD")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		}
		return
	}

	// 2. /api/v1/topology/summary
	if path == "summary" {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			return
		}
		s.handleGetTopologySummary(w, r, topoSvc)
		return
	}

	// 3. /api/v1/topology/dot
	if path == "dot" {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			return
		}
		s.handleGetTopologyDOT(w, r, topoSvc)
		return
	}

	// 4. /api/v1/topology/path
	if path == "path" {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			return
		}
		s.handleGetTopologyPath(w, r, topoSvc)
		return
	}

	// 5. /api/v1/topology/spof or /api/v1/topology/spof/{id}
	if subPath, ok := strings.CutPrefix(path, "spof"); ok {
		subPath = strings.TrimPrefix(subPath, "/")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			return
		}
		if subPath == "" {
			s.handleGetTopologyAllSPOFs(w, r, topoSvc)
		} else {
			s.handleGetTopologyNodeSPOF(w, r, topoSvc, subPath)
		}
		return
	}

	// 6. /api/v1/topology/impact/{id}
	if subPath, ok := strings.CutPrefix(path, "impact"); ok {
		subPath = strings.TrimPrefix(subPath, "/")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			return
		}
		if subPath == "" {
			s.writeAPIError(w, r, http.StatusBadRequest, "MISSING_NODE_ID", "Node ID is required for impact analysis")
			return
		}
		s.handleGetTopologyNodeImpact(w, r, topoSvc, subPath)
		return
	}

	// 7. /api/v1/topology/nodes or /api/v1/topology/nodes/{id}
	if subPath, ok := strings.CutPrefix(path, "nodes"); ok {
		subPath = strings.TrimPrefix(subPath, "/")
		if subPath == "" {
			switch r.Method {
			case http.MethodPost:
				s.handlePostTopologyNode(w, r, topoSvc)
			default:
				w.Header().Set("Allow", "POST")
				s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			}
		} else {
			switch r.Method {
			case http.MethodGet, http.MethodHead:
				s.handleGetTopologyNode(w, r, topoSvc, subPath)
			case http.MethodDelete:
				s.handleDeleteTopologyNode(w, r, topoSvc, subPath)
			default:
				w.Header().Set("Allow", "GET, HEAD, DELETE")
				s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			}
		}
		return
	}

	// 8. /api/v1/topology/dependencies or /api/v1/topology/dependencies/{id}
	if subPath, ok := strings.CutPrefix(path, "dependencies"); ok {
		subPath = strings.TrimPrefix(subPath, "/")
		if subPath == "" {
			switch r.Method {
			case http.MethodPost:
				s.handlePostTopologyDependency(w, r, topoSvc)
			case http.MethodDelete:
				s.handleDeleteTopologyDependency(w, r, topoSvc)
			default:
				w.Header().Set("Allow", "POST, DELETE")
				s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			}
		} else {
			switch r.Method {
			case http.MethodGet, http.MethodHead:
				s.handleGetTopologyDependencies(w, r, topoSvc, subPath)
			default:
				w.Header().Set("Allow", "GET, HEAD")
				s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			}
		}
		return
	}

	// 9. /api/v1/topology/dependents/{id}
	if subPath, ok := strings.CutPrefix(path, "dependents"); ok {
		subPath = strings.TrimPrefix(subPath, "/")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			return
		}
		if subPath == "" {
			s.writeAPIError(w, r, http.StatusBadRequest, "MISSING_NODE_ID", "Node ID is required for dependents query")
			return
		}
		s.handleGetTopologyDependents(w, r, topoSvc, subPath)
		return
	}

	s.writeAPIError(w, r, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("Path /api/v1/topology/%s not found", path))
}

func (s *Server) handleGetTopology(w http.ResponseWriter, r *http.Request, topoSvc topology.Service) {
	q := r.URL.Query()
	filter := topology.TopologyFilter{
		Source: q.Get("source"),
		HostID: q.Get("host_id"),
		Search: q.Get("search"),
	}

	if types := q["type"]; len(types) > 0 {
		for _, t := range types {
			for _, item := range strings.Split(t, ",") {
				trimmed := strings.TrimSpace(item)
				if trimmed != "" {
					filter.Types = append(filter.Types, topology.NodeType(trimmed))
				}
			}
		}
	}

	if statuses := q["status"]; len(statuses) > 0 {
		for _, st := range statuses {
			for _, item := range strings.Split(st, ",") {
				trimmed := strings.TrimSpace(item)
				if trimmed != "" {
					filter.Statuses = append(filter.Statuses, topology.NodeStatus(trimmed))
				}
			}
		}
	}

	if maxDepthStr := q.Get("max_depth"); maxDepthStr != "" {
		if md, err := strconv.Atoi(maxDepthStr); err == nil && md > 0 {
			filter.MaxDepth = md
		}
	}

	tagFilter := make(map[string]string)
	for k, v := range q {
		if strings.HasPrefix(k, "tag:") && len(v) > 0 {
			tagName := strings.TrimPrefix(k, "tag:")
			tagFilter[tagName] = v[0]
		}
	}
	if len(tagFilter) > 0 {
		filter.TagFilter = tagFilter
	}

	resp := topoSvc.GetTopology(filter)
	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetTopologySummary(w http.ResponseWriter, _ *http.Request, topoSvc topology.Service) {
	resp := topoSvc.GetTopology(topology.TopologyFilter{})
	s.writeJSON(w, http.StatusOK, resp.Summary)
}

func (s *Server) handleGetTopologyDOT(w http.ResponseWriter, _ *http.Request, topoSvc topology.Service) {
	dot := topoSvc.ExportDOT()
	w.Header().Set("Content-Type", "text/vnd.graphviz; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(dot))
}

func (s *Server) handleGetTopologyPath(w http.ResponseWriter, r *http.Request, topoSvc topology.Service) {
	q := r.URL.Query()
	source := q.Get("source")
	if source == "" {
		source = q.Get("source_id")
	}
	target := q.Get("target")
	if target == "" {
		target = q.Get("target_id")
	}

	if source == "" || target == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, "MISSING_PARAMETER", "Both 'source' and 'target' query parameters are required")
		return
	}

	path, found := topoSvc.FindPath(source, target)
	if !found {
		s.writeAPIError(w, r, http.StatusNotFound, "PATH_NOT_FOUND", fmt.Sprintf("No dependency path found from '%s' to '%s'", source, target))
		return
	}

	s.writeJSON(w, http.StatusOK, path)
}

func (s *Server) handleGetTopologyAllSPOFs(w http.ResponseWriter, r *http.Request, topoSvc topology.Service) {
	minCrit := 0.0
	if minCritStr := r.URL.Query().Get("min_criticality"); minCritStr != "" {
		if val, err := strconv.ParseFloat(minCritStr, 64); err == nil {
			minCrit = val
		}
	}

	spofs := topoSvc.FindAllSPOFs(minCrit)
	s.writeJSON(w, http.StatusOK, map[string]any{
		"spofs":        spofs,
		"count":        len(spofs),
		"evaluated_at": time.Now().UTC(),
	})
}

func (s *Server) handleGetTopologyNodeSPOF(w http.ResponseWriter, r *http.Request, topoSvc topology.Service, nodeID string) {
	spof := topoSvc.AnalyzeSPOF(nodeID)
	if spof == nil {
		s.writeAPIError(w, r, http.StatusNotFound, "NODE_NOT_FOUND", fmt.Sprintf("Topology node '%s' not found for SPOF analysis", nodeID))
		return
	}

	s.writeJSON(w, http.StatusOK, spof)
}

func (s *Server) handleGetTopologyNodeImpact(w http.ResponseWriter, r *http.Request, topoSvc topology.Service, nodeID string) {
	impact := topoSvc.AnalyzeImpact(nodeID)
	if impact == nil {
		s.writeAPIError(w, r, http.StatusNotFound, "NODE_NOT_FOUND", fmt.Sprintf("Topology node '%s' not found for impact analysis", nodeID))
		return
	}

	s.writeJSON(w, http.StatusOK, impact)
}

func (s *Server) handleGetTopologyNode(w http.ResponseWriter, r *http.Request, topoSvc topology.Service, nodeID string) {
	node, exists := topoSvc.GetNode(nodeID)
	if !exists {
		s.writeAPIError(w, r, http.StatusNotFound, "NODE_NOT_FOUND", fmt.Sprintf("Topology node '%s' not found", nodeID))
		return
	}

	s.writeJSON(w, http.StatusOK, node)
}

func (s *Server) handleGetTopologyDependencies(w http.ResponseWriter, r *http.Request, topoSvc topology.Service, nodeID string) {
	if _, exists := topoSvc.GetNode(nodeID); !exists {
		s.writeAPIError(w, r, http.StatusNotFound, "NODE_NOT_FOUND", fmt.Sprintf("Topology node '%s' not found", nodeID))
		return
	}

	deps := topoSvc.GetDependencies(nodeID)
	s.writeJSON(w, http.StatusOK, map[string]any{
		"node_id":      nodeID,
		"dependencies": deps,
		"count":        len(deps),
	})
}

func (s *Server) handleGetTopologyDependents(w http.ResponseWriter, r *http.Request, topoSvc topology.Service, nodeID string) {
	if _, exists := topoSvc.GetNode(nodeID); !exists {
		s.writeAPIError(w, r, http.StatusNotFound, "NODE_NOT_FOUND", fmt.Sprintf("Topology node '%s' not found", nodeID))
		return
	}

	dependents := topoSvc.GetDependents(nodeID)
	s.writeJSON(w, http.StatusOK, map[string]any{
		"node_id":    nodeID,
		"dependents": dependents,
		"count":      len(dependents),
	})
}

func (s *Server) handlePostTopologyNode(w http.ResponseWriter, r *http.Request, topoSvc topology.Service) {
	var node topology.TopologyNode
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_PAYLOAD", fmt.Sprintf("Failed to decode JSON payload: %v", err))
		return
	}

	if strings.TrimSpace(node.ID) == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_NODE_ID", "Node ID cannot be empty")
		return
	}

	if node.Name == "" {
		node.Name = node.ID
	}
	if node.Type == "" {
		node.Type = topology.NodeTypeService
	}
	if node.Status == "" {
		node.Status = topology.NodeStatusHealthy
	}

	topoSvc.AddDeclaredNode(node)
	s.writeJSON(w, http.StatusCreated, node)
}

func (s *Server) handlePostTopologyDependency(w http.ResponseWriter, r *http.Request, topoSvc topology.Service) {
	var dep topology.Dependency
	if err := json.NewDecoder(r.Body).Decode(&dep); err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_PAYLOAD", fmt.Sprintf("Failed to decode JSON payload: %v", err))
		return
	}

	if strings.TrimSpace(dep.SourceID) == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_SOURCE_ID", "Dependency source_id cannot be empty")
		return
	}
	if strings.TrimSpace(dep.TargetID) == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_TARGET_ID", "Dependency target_id cannot be empty")
		return
	}
	if dep.Type == "" {
		dep.Type = topology.RelDependsOn
	}
	if dep.Confidence == "" {
		dep.Confidence = topology.ConfidenceHigh
	}
	if dep.Weight <= 0 {
		dep.Weight = 1.0
	}

	topoSvc.AddDeclaredDependency(dep)
	s.writeJSON(w, http.StatusCreated, dep)
}

func (s *Server) handleDeleteTopologyNode(w http.ResponseWriter, r *http.Request, topoSvc topology.Service, nodeID string) {
	removed := topoSvc.RemoveNode(nodeID)
	if !removed {
		s.writeAPIError(w, r, http.StatusNotFound, "NODE_NOT_FOUND", fmt.Sprintf("Topology node '%s' not found", nodeID))
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"deleted":   true,
		"node_id":   nodeID,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleDeleteTopologyDependency(w http.ResponseWriter, r *http.Request, topoSvc topology.Service) {
	q := r.URL.Query()
	sourceID := q.Get("source")
	if sourceID == "" {
		sourceID = q.Get("source_id")
	}
	targetID := q.Get("target")
	if targetID == "" {
		targetID = q.Get("target_id")
	}
	relType := topology.RelationshipType(q.Get("type"))

	// If not in query params, attempt to parse JSON body
	if sourceID == "" || targetID == "" {
		var body struct {
			SourceID string                  `json:"source_id"`
			TargetID string                  `json:"target_id"`
			Type     topology.RelationshipType `json:"type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			if sourceID == "" {
				sourceID = body.SourceID
			}
			if targetID == "" {
				targetID = body.TargetID
			}
			if relType == "" {
				relType = body.Type
			}
		}
	}

	if sourceID == "" || targetID == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, "MISSING_PARAMETER", "Both 'source' and 'target' are required to delete a dependency")
		return
	}

	removed := topoSvc.RemoveDependency(sourceID, targetID, relType)
	if !removed {
		s.writeAPIError(w, r, http.StatusNotFound, "DEPENDENCY_NOT_FOUND", fmt.Sprintf("Dependency between '%s' and '%s' not found", sourceID, targetID))
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"deleted":   true,
		"source_id": sourceID,
		"target_id": targetID,
		"type":      relType,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
