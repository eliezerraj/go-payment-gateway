package api

import (
	"fmt"
	"time"
	"context"
	"encoding/json"
	"reflect"
	"net/http"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/go-payment-gateway/internal/core/service"
	"github.com/go-payment-gateway/internal/core/model"
	"github.com/go-payment-gateway/internal/core/erro"

	"github.com/eliezerraj/go-core/coreJson"
	"github.com/gorilla/mux"

	go_core_observ "github.com/eliezerraj/go-core/observability"
	go_core_tools "github.com/eliezerraj/go-core/tools"
)

var childLogger = log.With().Str("component", "go-payment-gateway").Str("package", "internal.adapter.api").Logger()

var core_json 		coreJson.CoreJson
var core_apiError 	coreJson.APIError
var core_tools 		go_core_tools.ToolsCore
var tracerProvider 	go_core_observ.TracerProvider

type HttpRouters struct {
	workerService 	*service.WorkerService
	ctxTimeout		time.Duration
}

// Above create routers
func NewHttpRouters(workerService *service.WorkerService,
					ctxTimeout	time.Duration) HttpRouters {
	childLogger.Info().Str("func","NewHttpRouters").Send()

	return HttpRouters{
		workerService: workerService,
		ctxTimeout: ctxTimeout,
	}
}

// About return a health
func (h *HttpRouters) Health(rw http.ResponseWriter, req *http.Request) {
	childLogger.Info().Str("func","Health").Send()

	json.NewEncoder(rw).Encode(model.MessageRouter{Message: "true"})
}

// About return a live
func (h *HttpRouters) Live(rw http.ResponseWriter, req *http.Request) {
	childLogger.Info().Str("func","Live").Send()

	json.NewEncoder(rw).Encode(model.MessageRouter{Message: "true"})
}

// About show all header received
func (h *HttpRouters) Header(rw http.ResponseWriter, req *http.Request) {
	childLogger.Info().Str("func","Header").Interface("trace-resquest-id", req.Context().Value("trace-request-id")).Send()
	
	json.NewEncoder(rw).Encode(req.Header)
}

// About show all context values
func (h *HttpRouters) Context(rw http.ResponseWriter, req *http.Request) {
	childLogger.Info().Str("func","Context").Interface("trace-resquest-id", req.Context().Value("trace-request-id")).Send()
	
	contextValues := reflect.ValueOf(req.Context()).Elem()
	json.NewEncoder(rw).Encode(fmt.Sprintf("%v",contextValues))
}

// About show pgx stats
func (h *HttpRouters) Stat(rw http.ResponseWriter, req *http.Request) {
	childLogger.Info().Str("func","Stat").Interface("trace-resquest-id", req.Context().Value("trace-request-id")).Send()
	
	res := h.workerService.Stat(req.Context())

	json.NewEncoder(rw).Encode(res)
}

// About handle error
func (h *HttpRouters) ErrorHandler(trace_id string, err error) *coreJson.APIError {
	if strings.Contains(err.Error(), "context deadline exceeded") {
    	err = erro.ErrTimeout
	} 
	switch err {
	case erro.ErrBadRequest:
		core_apiError = core_apiError.NewAPIError(err, trace_id, http.StatusBadRequest)
	case erro.ErrNotFound:
		core_apiError = core_apiError.NewAPIError(err, trace_id, http.StatusNotFound)
	case erro.ErrTimeout:
		core_apiError = core_apiError.NewAPIError(err, trace_id, http.StatusGatewayTimeout)
	default:
		core_apiError = core_apiError.NewAPIError(err, trace_id, http.StatusInternalServerError)
	}
	return &core_apiError
}

// About do payment
func (h *HttpRouters) AddPayment(rw http.ResponseWriter, req *http.Request) error {
	childLogger.Info().Str("func","AddPayment").Interface("trace-resquest-id", req.Context().Value("trace-request-id")).Send()

	ctx, cancel := context.WithTimeout(req.Context(), h.ctxTimeout * time.Second)
    defer cancel()

	span := tracerProvider.Span(ctx, "adapter.api.AddPayment")
	defer span.End()
	
	trace_id := fmt.Sprintf("%v", ctx.Value("trace-request-id"))

	payment := model.Payment{}
	err := json.NewDecoder(req.Body).Decode(&payment)
    if err != nil {
		return h.ErrorHandler(trace_id, erro.ErrBadRequest)
    }
	defer req.Body.Close()

	res, err := h.workerService.AddPayment(ctx, payment)
	if err != nil {
		return h.ErrorHandler(trace_id, err)
	}
	
	return core_json.WriteJSON(rw, http.StatusOK, res)
}

// About add a pix moviment transaction
func (h *HttpRouters) PixTransaction(rw http.ResponseWriter, req *http.Request) error {
	childLogger.Info().Str("func","PixTransaction").Interface("trace-resquest-id", req.Context().Value("trace-request-id")).Send()

	ctx, cancel := context.WithTimeout(req.Context(), h.ctxTimeout * time.Second)
    defer cancel()

	span := tracerProvider.Span(ctx, "adapter.api.PixTransaction")
	defer span.End()
	
	trace_id := fmt.Sprintf("%v", ctx.Value("trace-request-id"))

	pixTransaction := model.PixTransaction{}
	err := json.NewDecoder(req.Body).Decode(&pixTransaction)
    if err != nil {
		return h.ErrorHandler(trace_id, erro.ErrBadRequest)
    }
	defer req.Body.Close()

	res, err := h.workerService.PixTransaction(ctx, pixTransaction)
	if err != nil {
		return h.ErrorHandler(trace_id, err)
	}
	
	return core_json.WriteJSON(rw, http.StatusOK, res)
}

// About get a pix transaction
func (h *HttpRouters) GetPixTransaction(rw http.ResponseWriter, req *http.Request) error {
	childLogger.Info().Str("func","GetPixTransaction").Interface("trace-resquest-id", req.Context().Value("trace-request-id")).Send()

	ctx, cancel := context.WithTimeout(req.Context(), h.ctxTimeout * time.Second)
    defer cancel()

	span := tracerProvider.Span(ctx, "adapter.api.GetPixTransaction")
	defer span.End()
	
	trace_id := fmt.Sprintf("%v", ctx.Value("trace-request-id"))

	vars := mux.Vars(req)
	varID := vars["id"]

	varIDint, err := strconv.Atoi(varID)
    if err != nil {
		return h.ErrorHandler(trace_id, erro.ErrBadRequest)
    }

	pixTransaction := model.PixTransaction{ID: varIDint}

	res, err := h.workerService.GetPixTransaction(ctx, pixTransaction)
	if err != nil {
		return h.ErrorHandler(trace_id, err)
	}
	
	return core_json.WriteJSON(rw, http.StatusOK, res)
}

// About get a pix transaction
func (h *HttpRouters) StatPixTransaction(rw http.ResponseWriter, req *http.Request) error {
	childLogger.Info().Str("func","StatPixTransaction").Interface("trace-resquest-id", req.Context().Value("trace-request-id")).Send()

	ctx, cancel := context.WithTimeout(req.Context(), h.ctxTimeout * time.Second)
    defer cancel()

	span := tracerProvider.Span(ctx, "adapter.api.StatPixTransaction")
	defer span.End()
	
	trace_id := fmt.Sprintf("%v", ctx.Value("trace-request-id"))

	vars := mux.Vars(req)
	varID := vars["id"]

	pixStatusAccount := model.PixStatusAccount{AccountFrom: varID}

	res, err := h.workerService.StatPixTransaction(ctx, pixStatusAccount)
	if err != nil {
		return h.ErrorHandler(trace_id, err)
	}
	
	return core_json.WriteJSON(rw, http.StatusOK, res)
}