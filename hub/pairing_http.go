package main

import (
	"encoding/json"
	"net/http"
)

type PairRequestPayload struct {
	Name string `json:"name"`
}

type PairDecisionPayload struct {
	ID string `json:"id"`
}

func (server *HubServer) registerPairingRoutes(mux *http.ServeMux) {
	pairing := server.pairingManager
	mux.HandleFunc("/api/hub/ping", server.handlePing)
	mux.HandleFunc("/api/hub/pair/request", server.handlePairRequest)
	mux.HandleFunc("/api/hub/pair/status", server.handlePairStatus)
	mux.HandleFunc("/api/hub/pair/list", pairing.RequireWebPage(server.handlePairList))
	mux.HandleFunc("/api/hub/pair/approve", pairing.RequireWebPage(server.handlePairApprove))
	mux.HandleFunc("/api/hub/pair/reject", pairing.RequireWebPage(server.handlePairReject))
	mux.HandleFunc("/api/hub/pair/revoke", pairing.RequireWebPage(server.handlePairRevoke))
}

func (server *HubServer) handlePing(writer http.ResponseWriter, request *http.Request) {
	server.writeJSON(writer, map[string]interface{}{"ok": true, "hub": "freetv"}, http.StatusOK)
}

func (server *HubServer) handlePairRequest(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload PairRequestPayload
	_ = json.NewDecoder(request.Body).Decode(&payload)

	pairRequest, requestErr := server.pairingManager.Request(payload.Name)
	if requestErr != nil {
		server.writeJSON(writer, errorBody(requestErr), http.StatusTooManyRequests)
		return
	}

	server.writeJSON(writer, map[string]interface{}{
		"ok":   true,
		"id":   pairRequest.ID,
		"code": pairRequest.Code,
	}, http.StatusOK)
}

func (server *HubServer) handlePairStatus(writer http.ResponseWriter, request *http.Request) {
	status, token, statusErr := server.pairingManager.Status(request.URL.Query().Get("id"))
	if statusErr != nil {
		server.writeJSON(writer, errorBody(statusErr), http.StatusNotFound)
		return
	}

	server.writeJSON(writer, map[string]interface{}{
		"ok":     true,
		"status": status,
		"token":  token,
	}, http.StatusOK)
}

func (server *HubServer) handlePairList(writer http.ResponseWriter, request *http.Request) {
	waiting, devices := server.pairingManager.Snapshot()
	server.writeJSON(writer, map[string]interface{}{
		"ok":      true,
		"pending": waiting,
		"paired":  devices,
	}, http.StatusOK)
}

func (server *HubServer) handlePairApprove(writer http.ResponseWriter, request *http.Request) {
	server.decidePairing(writer, request, server.pairingManager.Approve)
}

func (server *HubServer) handlePairReject(writer http.ResponseWriter, request *http.Request) {
	server.decidePairing(writer, request, server.pairingManager.Reject)
}

func (server *HubServer) handlePairRevoke(writer http.ResponseWriter, request *http.Request) {
	server.decidePairing(writer, request, server.pairingManager.Revoke)
}

func (server *HubServer) decidePairing(
	writer http.ResponseWriter,
	request *http.Request,
	decide func(string) error,
) {
	if request.Method != http.MethodPost {
		http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload PairDecisionPayload
	_ = json.NewDecoder(request.Body).Decode(&payload)

	if decideErr := decide(payload.ID); decideErr != nil {
		server.writeJSON(writer, errorBody(decideErr), http.StatusBadRequest)
		return
	}

	server.writeJSON(writer, map[string]interface{}{"ok": true}, http.StatusOK)
}

func errorBody(failure error) map[string]interface{} {
	return map[string]interface{}{"ok": false, "error": failure.Error()}
}
