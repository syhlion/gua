package restresp

import (
	"encoding/json"
	"net/http"
)

//Write http response write restful api style
func Write(w http.ResponseWriter, data interface{}, httpStatus int) {
	_, ok := data.(error)
	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	w.WriteHeader(httpStatus)
	if ok && httpStatus == http.StatusOK {
		json.NewEncoder(w).Encode(struct {
			Error interface{} `json:"error"`
		}{data})
		return
	}
	if httpStatus == http.StatusOK {
		json.NewEncoder(w).Encode(struct {
			Success interface{} `json:"success"`
		}{data})
		return
	}
	json.NewEncoder(w).Encode(struct {
		Error interface{} `json:"error"`
	}{data})
	return

}
