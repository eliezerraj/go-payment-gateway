package service

import(
	"fmt"
	"time"
	"context"
	"strconv"
	"errors"
	"net/http"
	"encoding/json"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/go-payment-gateway/internal/core/model"
	"github.com/go-payment-gateway/internal/core/erro"
	"github.com/go-payment-gateway/internal/adapter/database"
	"github.com/go-payment-gateway/internal/adapter/event"

	go_core_pg "github.com/eliezerraj/go-core/database/pg"
	go_core_observ "github.com/eliezerraj/go-core/observability"
	go_core_api "github.com/eliezerraj/go-core/api"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

var (
	tracerProvider go_core_observ.TracerProvider
	childLogger = log.With().Str("component","go-payment-gateway").Str("package","internal.core.service").Logger()
	apiService go_core_api.ApiService
)

type WorkerService struct {
	goCoreRestApiService	go_core_api.ApiService
	apiService				[]model.ApiService
	workerRepository 		*database.WorkerRepository
	workerEvent				*event.WorkerEvent
	mutex    				sync.Mutex // mutex to avoid commit over a transaction on air for kafka message
}

// About create a new worker service
func NewWorkerService(	goCoreRestApiService	go_core_api.ApiService,	
						workerRepository *database.WorkerRepository,
						apiService		[]model.ApiService,
						workerEvent	*event.WorkerEvent,) *WorkerService{
	childLogger.Info().Str("func","NewWorkerService").Send()

	return &WorkerService{
		goCoreRestApiService: goCoreRestApiService,
		apiService: apiService,
		workerRepository: workerRepository,
		workerEvent: workerEvent,
	}
}

// About handle/convert http status code
func errorStatusCode(statusCode int, serviceName string) error{
	childLogger.Info().Str("func","errorStatusCode").Interface("serviceName", serviceName).Interface("statusCode", statusCode).Send()
	var err error
	switch statusCode {
		case http.StatusUnauthorized:
			err = erro.ErrUnauthorized
		case http.StatusForbidden:
			err = erro.ErrHTTPForbiden
		case http.StatusNotFound:
			err = erro.ErrNotFound
		default:
			err = errors.New(fmt.Sprintf("service %s in outage", serviceName))
		}
	return err
}

// About handle/convert http status code
func (s *WorkerService) Stat(ctx context.Context) (go_core_pg.PoolStats){
	childLogger.Info().Str("func","Stat").Interface("trace-resquest-id", ctx.Value("trace-request-id")).Send()

	return s.workerRepository.Stat(ctx)
}

// About step process
func appentStepProcess(nameStepProcess string, listStepProcess []model.StepProcess) ([]model.StepProcess) {
	
	stepProcess := model.StepProcess{	Name: nameStepProcess,
										ProcessedAt: time.Now(),}
	
	return append(listStepProcess, stepProcess) 
}

// About create a payment data
func (s * WorkerService) AddPayment(ctx context.Context, payment model.Payment) (*model.Payment, error){
	childLogger.Info().Str("func","AddPayment").Interface("trace-request-id", ctx.Value("trace-request-id")).Interface("payment", payment).Send()

	// Trace
	ctx, span := tracerProvider.SpanCtx(ctx, "service.AddPayment")
	trace_id := fmt.Sprintf("%v",ctx.Value("trace-request-id"))

	// Get the database connection
	tx, conn, err := s.workerRepository.DatabasePGServer.StartTx(ctx)
	if err != nil {
		return nil, err
	}
	defer s.workerRepository.DatabasePGServer.ReleaseTx(conn)

	// Handle the transaction
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		} else {
			tx.Commit(ctx)
		}	
		span.End()
	}()

	// Businness rule
	if (payment.CardType != "CREDIT") && (payment.CardType != "DEBIT") {
		return nil, erro.ErrCardTypeInvalid
	}

	if payment.TransactionId == nil  {
		return nil, erro.ErrTransactioInvalid
	}

	// Get terminal
	terminal := model.Terminal{Name: payment.Terminal}
	res_terminal, err := s.workerRepository.GetTerminal(ctx, terminal)
	if err != nil {
		return nil, err
	}

	// ------------------------  STEP-1 ----------------------------------//
	childLogger.Info().Str("func","AddPayment").Msg("===> STEP-1 (PAYMENT) <===")
	stepctx1, stepSpan1 := tracerProvider.SpanCtx(ctx, "service.AddPayment:STEP-1-(payment)")

	// prepare headers
	headers := map[string]string{
		"Content-Type":  "application/json;charset=UTF-8",
		"X-Request-Id": trace_id,
		"x-apigw-api-id": s.apiService[3].XApigwApiId,
		"Host": s.apiService[3].HostName,
	}
	// prepare http client
	httpClient := go_core_api.HttpClient {
		Url: fmt.Sprintf("%s/%s/%s",s.apiService[3].Url,"card",payment.CardNumber),
		Method: s.apiService[3].Method,
		Timeout: s.apiService[3].HttpTimeout,
		Headers: &headers,
	}

	// get card data
	res_payload, statusCode, err := apiService.CallRestApiV1(stepctx1,
															s.goCoreRestApiService.Client,
															httpClient, 
															nil)

	if err != nil {
		return nil, errorStatusCode(statusCode, s.apiService[3].Name)
	}
	jsonString, err  := json.Marshal(res_payload)
	if err != nil {
		return nil, errors.New(err.Error())
    }
	var card_parsed model.Card
	json.Unmarshal(jsonString, &card_parsed)

	// Prepare payment
	payment.FkCardId = card_parsed.ID
	payment.CardNumber = card_parsed.CardNumber
	payment.CardModel = card_parsed.Model
	payment.CardType = card_parsed.Type
	payment.FkTerminalId = res_terminal.ID
	payment.RequestId = &trace_id
	payment.Status = "AUTHORIZATION:PENDING"

	// create a payment
	res_payment, err := s.workerRepository.AddPayment(stepctx1, tx, &payment)
	if err != nil {
		return nil, err
	}
	payment.ID = res_payment.ID // Set PK

	// Create a StepProcess
	list_stepProcess := []model.StepProcess{}
	var stepProcessStatus string
	
	stepProcessStatus = "AUTHORIZATION:STATUS:PENDING"
	list_stepProcess = appentStepProcess(stepProcessStatus, list_stepProcess)
	
	stepSpan1.End()
	// ------------------------  STEP-2 ----------------------------------//
	childLogger.Info().Str("func","AddPayment").Msg("===> STEP-2 (LIMIT) <===")
	stepctx2, stepSpan2 := tracerProvider.SpanCtx(ctx, "service.AddPayment:STEP-2-(limit)")
	
	// Check the limits
	limit := model.Limit{ 	TransactionId: *payment.TransactionId,
						  	Key: 	payment.CardNumber,
							TypeLimit: "CREDIT",
							OrderLimit: "MCC:" + payment.Mcc,
							Amount:	payment.Amount,
							Quantity: 1}

	// Set headers
	headers = map[string]string{
		"Content-Type": "application/json",
		"X-Request-Id": trace_id,
		"x-apigw-api-id": s.apiService[1].XApigwApiId,
		"Host": s.apiService[1].HostName,
	}
	// Prepare http client
	httpClient = go_core_api.HttpClient {
		Url: fmt.Sprintf("%v%v",s.apiService[1].Url,"/checkLimitTransaction"),
		Method: s.apiService[1].Method,
		Timeout: s.apiService[1].HttpTimeout,
		Headers: &headers,
	}

	// Call go-limit
	res_limit, statusCode, err := apiService.CallRestApiV1(	stepctx2,
															s.goCoreRestApiService.Client,	
															httpClient, 
															limit)

	if err != nil {
		return nil, errorStatusCode(statusCode, s.apiService[1].Name)
	}

	list_limit_transaction := []model.LimitTransaction{}
	jsonString, err = json.Marshal(res_limit)
	if err != nil {
		return nil, errors.New(err.Error())
    }
	json.Unmarshal(jsonString, &list_limit_transaction)
	
	var list_status = []string{}
	for _, val := range list_limit_transaction {
		list_status = append(list_status, val.Status)
	}

	// add step 02
	stepProcessStatus = fmt.Sprintf("LIMIT:%v", list_status)
	list_stepProcess = appentStepProcess(stepProcessStatus, list_stepProcess)
	
	stepSpan2.End()

	// ------------------------  STEP-3 ----------------------------------//
	childLogger.Info().Str("func","AddPayment").Msg("===> STEP-3 (FRAUD) <===")
	// Check Fraud

	// ------------------------  STEP-4 ----------------------------------//
	childLogger.Info().Str("func","AddPayment").Msg("===> STEP-4 (LEDGER) <===")
	stepctx4, stepSpan4 := tracerProvider.SpanCtx(ctx, "service.AddPayment:STEP-4-(ledger)")

	// Access Account (ledger)
	moviment := model.Moviment{	AccountID: card_parsed.AccountID,
								AccountFrom: model.Account{AccountID: card_parsed.AccountID},
								Type: "WITHDRAW",
								Currency: payment.Currency,
								Amount: payment.Amount }

	// Set headers
	headers = map[string]string{
		"Content-Type":  "application/json;charset=UTF-8",
		"X-Request-Id": trace_id,
		"x-apigw-api-id": s.apiService[2].XApigwApiId,
		"Host": s.apiService[2].HostName,
	}
	// prepare http client
	httpClient = go_core_api.HttpClient {
		Url: 	s.apiService[2].Url + "/movimentTransaction",
		Method: s.apiService[2].Method,
		Timeout: s.apiService[2].HttpTimeout,
		Headers: &headers,
	}

	// Call go-ledger
	_, statusCode, err = apiService.CallRestApiV1(stepctx4,
												s.goCoreRestApiService.Client,
												httpClient, 
												moviment)
	if err != nil {
		return nil, errorStatusCode(statusCode, s.apiService[2].Name)
	}

	// add step 04
	stepProcessStatus = "LEDGER:WITHDRAW:OK"
	list_stepProcess = appentStepProcess(stepProcessStatus, list_stepProcess)
	stepSpan4.End()

	// ------------------------  STEP-5 ----------------------------------//
	childLogger.Info().Str("func","AddPayment").Msg("===> STEP-5 (CARDS:ATC) <===")
	stepctx5, stepSpan5 := tracerProvider.SpanCtx(ctx, "service.AddPayment:STEP-5-(cards-atc)")

	// prepare headers
	headers = map[string]string{
		"Content-Type":  "application/json;charset=UTF-8",
		"X-Request-Id": trace_id,
		"x-apigw-api-id": s.apiService[4].XApigwApiId,
		"Host": s.apiService[4].HostName,
	}
	// prepare http client
	httpClient = go_core_api.HttpClient {
		Url: fmt.Sprintf("%s/%s",s.apiService[4].Url,"atc"),
		Method: s.apiService[4].Method,
		Timeout: s.apiService[4].HttpTimeout,
		Headers: &headers,
	}

	// get card data
	res_payload, statusCode, err = apiService.CallRestApiV1(stepctx5,
															s.goCoreRestApiService.Client,
															httpClient, 
															card_parsed)
	if err != nil {
		return nil, errorStatusCode(statusCode, s.apiService[3].Name)
	}

	stepProcessStatus = "CARD-ATC:OK"
	list_stepProcess = appentStepProcess(stepProcessStatus, list_stepProcess)
	stepSpan5.End()

	// ------------------------  STEP-6 ----------------------------------//
	childLogger.Info().Str("func","AddPayment").Msg("===> STEP-6 (UPDATE PAYMENT) <===")
	stepctx6, stepSpan6 := tracerProvider.SpanCtx(ctx, "service.AddPayment:STEP-6-(update-payment-status)")

	// update status payment
	payment.Status = "AUTHORIZATION:OK"
	res_update, err := s.workerRepository.UpdatePayment(stepctx6, tx, payment)
	if err != nil {
		return nil, err
	}
	if res_update == 0 {
		err = erro.ErrUpdate
		return nil, err
	}

	stepProcessStatus = "AUTHORIZATION:STATUS:OK"
	list_stepProcess = appentStepProcess(stepProcessStatus, list_stepProcess)

	childLogger.Info().Str("func","AddPayment: ===> FINAL").Interface("payment", payment).Send()
	
	// add the step proccess
	payment.StepProcess = &list_stepProcess
	stepSpan6.End()

	return &payment, nil
}

// About create pix transaction
func (s * WorkerService) PixTransaction(ctx context.Context, pixTransaction model.PixTransaction) (*model.PixTransaction, error){
	childLogger.Info().Str("func","PixTransaction").Interface("trace-request-id", ctx.Value("trace-request-id")).Interface("pixTransaction", pixTransaction).Send()

	// Trace
	ctx, span := tracerProvider.SpanCtx(ctx, "service.PixTransaction")
	trace_id := fmt.Sprintf("%v",ctx.Value("trace-request-id"))

	// Get the database connection
	tx, conn, err := s.workerRepository.DatabasePGServer.StartTx(ctx)
	if err != nil {
		return nil, err
	}
	defer s.workerRepository.DatabasePGServer.ReleaseTx(conn)

	// handle connection
	defer func() {
		if err != nil {
			childLogger.Info().Interface("trace-resquest-id", trace_id ).Msg("ROLLBACK TX !!!")
			tx.Rollback(ctx)
		} else {
			childLogger.Info().Interface("trace-request-id", trace_id ).Msg("COMMIT TX !!!")
			tx.Commit(ctx)
		}
		span.End()
	}()

	// Create a StepProcess
	list_stepProcess := []model.StepProcess{}
	var stepProcessStatus string

	// ------------------------  STEP-1 ----------------------------------//
	childLogger.Info().Str("func","PixTransaction").Msg("===> STEP-1 (ACCOUNT FROM) <===")
	stepctx1, stepSpan1 := tracerProvider.SpanCtx(ctx, "service.PixTransaction:STEP-1-(account-from)")

	// prepare headers
	headers := map[string]string{
		"Content-Type":  	"application/json;charset=UTF-8",
		"X-Request-Id": 	trace_id,
		"x-apigw-api-id": 	s.apiService[0].XApigwApiId,
		"Host": 			s.apiService[0].HostName,
	}
	httpClient := go_core_api.HttpClient {
		Url: 	s.apiService[0].Url + "/get/" + pixTransaction.AccountFrom.AccountID,
		Method: s.apiService[0].Method,
		Timeout: s.apiService[0].HttpTimeout,
		Headers: &headers,
	}

	res_payload, statusCode, err := apiService.CallRestApiV1(stepctx1,
															s.goCoreRestApiService.Client,
															httpClient, 
															nil)
	if err != nil {
		return nil, errorStatusCode(statusCode, s.apiService[0].Name)
	}

	jsonString, err  := json.Marshal(res_payload)
	if err != nil {
		return nil, errors.New(err.Error())
    }
	var account_from_parsed model.Account
	json.Unmarshal(jsonString, &account_from_parsed)

	stepProcessStatus = "ACCOUNT-FROM:OK"
	list_stepProcess = appentStepProcess(stepProcessStatus, list_stepProcess)

	stepSpan1.End()
	// ------------------------  STEP-2 ----------------------------------//
	childLogger.Info().Str("func","PixTransaction").Msg("===> STEP-2 (ACCOUNT TO) <===")
	stepctx2, stepSpan2 := tracerProvider.SpanCtx(ctx, "service.PixTransaction:STEP-2-(account-to)")

	httpClient = go_core_api.HttpClient {
		Url: 	s.apiService[0].Url + "/get/" + pixTransaction.AccountTo.AccountID,
		Method: s.apiService[0].Method,
		Timeout: 15,
		Headers: &headers,
	}

	res_payload, statusCode, err = apiService.CallRestApiV1(stepctx2,
															s.goCoreRestApiService.Client,
															httpClient, 
															nil)
	if err != nil {
		return nil, errorStatusCode(statusCode, s.apiService[0].Name)
	}

	jsonString, err  = json.Marshal(res_payload)
	if err != nil {
		return nil, errors.New(err.Error())
    }
	var account_to_parsed model.Account
	json.Unmarshal(jsonString, &account_to_parsed)

	stepProcessStatus = "ACCOUNT-TO:OK"
	list_stepProcess = appentStepProcess(stepProcessStatus, list_stepProcess)
	stepSpan2.End()
	// ------------------------  STEP-3 ----------------------------------//
	childLogger.Info().Str("func","PixTransaction").Msg("===> STEP-3 (LIMIT) <===")
	stepctx3, stepSpan3 := tracerProvider.SpanCtx(ctx, "service.PixTransaction:STEP-3-(limit")
	
	// Check the limits
	limit := model.Limit{ 	TransactionId: pixTransaction.TransactionId,
						  	Key: account_from_parsed.AccountID + ":" + account_to_parsed.AccountID,
							TypeLimit: "TRANSFER",
							OrderLimit: "WIRE",
							Amount:	pixTransaction.Amount,
							Quantity: 1}

	// Set headers
	headers = map[string]string{
		"Content-Type": "application/json",
		"X-Request-Id": trace_id,
		"x-apigw-api-id": s.apiService[1].XApigwApiId,
		"Host": s.apiService[1].HostName,
	}
	// Prepare http client
	httpClient = go_core_api.HttpClient {
		Url: fmt.Sprintf("%v%v",s.apiService[1].Url,"/checkLimitTransaction"),
		Method: s.apiService[1].Method,
		Timeout: s.apiService[1].HttpTimeout,
		Headers: &headers,
	}

	// Call go-limit
	res_limit, statusCode, err := apiService.CallRestApiV1(	stepctx3,
															s.goCoreRestApiService.Client,	
															httpClient, 
															limit)

	if err != nil {
		return nil, errorStatusCode(statusCode, s.apiService[1].Name)
	}

	list_limit_transaction := []model.LimitTransaction{}
	jsonString, err = json.Marshal(res_limit)
	if err != nil {
		return nil, errors.New(err.Error())
    }
	json.Unmarshal(jsonString, &list_limit_transaction)
	
	var list_status = []string{}
	for _, val := range list_limit_transaction {
		list_status = append(list_status, val.Status)
	}

	stepProcessStatus = fmt.Sprintf("LIMIT:%v", list_status)
	list_stepProcess = appentStepProcess(stepProcessStatus, list_stepProcess)
	stepSpan3.End()

	// ------------------------  STEP-4 ----------------------------------//
	childLogger.Info().Str("func","PixTransaction").Msg("===> STEP-4 (PIX-TRANSACTION) <===")
	stepctx4, stepSpan4 := tracerProvider.SpanCtx(ctx, "service.PixTransaction:STEP-4-(pix-transaction)")

	pixTransaction.AccountFrom = account_from_parsed
	pixTransaction.AccountTo = account_to_parsed
	pixTransaction.Status = "PENDING"
	pixTransaction.RequestId = trace_id
	pixTransaction.TransactionAt = time.Now()
	
	// add pix payment
	res_pixTransaction, err := s.workerRepository.AddPixTransaction(stepctx4, tx, pixTransaction)
	if err != nil {
		return nil, err
	}
	// set the PK
	pixTransaction.ID = res_pixTransaction.ID
	pixTransaction.CreatedAt = res_pixTransaction.CreatedAt

	stepProcessStatus = "PIX-TRANSACTION:STATUS:PENDING"
	list_stepProcess = appentStepProcess(stepProcessStatus, list_stepProcess)
	stepSpan4.End()

	// ------------------------  STEP-5 ----------------------------------//
	childLogger.Info().Str("func","PixTransaction").Msg("===> STEP-5 (SEND TO LEDGE VIA MESSAGE - KAFKA) <===")
	stepctx5, stepSpan5 := tracerProvider.SpanCtx(ctx, "service.PixTransaction:STEP-5-(send-ledger-kafka)")

	stepProcessStatus = "LEDGER:WIRE-TRANSFER:IN-QUEUE"
	if s.workerEvent != nil {

		// Prepare to event
		key := strconv.Itoa(pixTransaction.ID)
		payload_bytes, err := json.Marshal(pixTransaction)
		if err != nil {
			return nil, err
		}

		err = s.ProducerEventKafka2(stepctx5, s.workerEvent.Topics[0], key, payload_bytes)
		if err != nil {
			return nil, err
		}
	}else {
		stepProcessStatus = "LEDGER:WIRE-TRANSFER:ERROR NOT SEND KAFKA UNREACHABLE"
	}

	list_stepProcess = appentStepProcess(stepProcessStatus, list_stepProcess)
	stepSpan5.End()

	// ------------------------  STEP-5 ----------------------------------//
	childLogger.Info().Str("func","PixTransaction").Msg("===> STEP-6 (UPDATE PIX-TRANSACTION <===")
	stepctx6, stepSpan6 := tracerProvider.SpanCtx(ctx, "service.PixTransaction:STEP-5-(update-pix-transaction)")

	// add the step proccess
	pixTransaction.StepProcess = &list_stepProcess

	// update status payment
	pixTransaction.Status = "IN-QUEUE:OK"
	res_update, err := s.workerRepository.UpdatePixTransaction(stepctx6, tx, pixTransaction)
	if err != nil {
		return nil, err
	}
	if res_update == 0 {
		err = erro.ErrUpdate
		return nil, err
	}

	stepProcessStatus = "PIX-TRANSACTION:STATUS:IN-QUEUE"
	list_stepProcess = appentStepProcess(stepProcessStatus, list_stepProcess)
	stepSpan6.End()

	// ------------------------  STEP-7 ----------------------------------//
	childLogger.Info().Str("func","PixTransaction").Msg("===> STEP-7 (WEBHOOK) <===")
	stepctx7, stepSpan7 := tracerProvider.SpanCtx(ctx, "service.PixTransaction:STEP-7-(webhook)")

	stepProcessStatus = "WEBHOOK:PAYMENT:IN-QUEUE"
	if s.workerEvent != nil {
		
		// Prepare to event
		payload_bytes, err := json.Marshal(pixTransaction)
		if err != nil {
			return nil, err
		}
		weebHook := model.WebHook{	Topic: s.workerEvent.Topics[1],
									Type: "TOPIC:PIX",
									Payload: payload_bytes}

		key := strconv.Itoa(pixTransaction.ID)
		payload_bytes, err = json.Marshal(weebHook)
		if err != nil {
			return nil, err
		}

		err = s.ProducerEventKafka2(stepctx7, s.workerEvent.Topics[1], key, payload_bytes)
		if err != nil {
			return nil, err
		}
	}else {
		stepProcessStatus = "WEBHOOK:PAYMENT:ERROR NOT SEND KAFKA UNREACHABLE"
	}

	list_stepProcess = appentStepProcess(stepProcessStatus, list_stepProcess)
	stepSpan7.End()

	childLogger.Info().Str("func","AddPayment: ===> FINAL").Interface("pixTransaction", pixTransaction).Send()

	return &pixTransaction, nil
}

// About handle/convert http status code
func (s *WorkerService) StatPixTransaction(ctx context.Context, pixStatusAccount model.PixStatusAccount) (*model.PixStatus, error){
	childLogger.Info().Str("func","StatPixTransaction").Interface("trace-resquest-id", ctx.Value("trace-request-id")).Send()

	// create a payment
	res_pixStatus, err := s.workerRepository.StatPixTransaction(ctx, pixStatusAccount)
	if err != nil {
		return nil, err
	}

	return res_pixStatus, nil
}

// About get pix info transaction
func (s *WorkerService) GetPixTransaction(ctx context.Context, pixTransaction model.PixTransaction) (*model.PixTransaction, error){
	childLogger.Info().Str("func","GetPixTransaction").Interface("trace-resquest-id", ctx.Value("trace-request-id")).Send()

	// get pix 
	res_pixTransaction, err := s.workerRepository.GetPixTransaction(ctx, pixTransaction)
	if err != nil {
		return nil, err
	}

	return res_pixTransaction, nil
}

//About producer a event in kafka
func(s *WorkerService) ProducerEventKafka2(ctx context.Context, topic string, keyMessage string, payloadBytes []byte) (err error) {
	childLogger.Info().Str("func","ProducerEventKafka").Interface("trace-request-id", ctx.Value("trace-request-id")).Send()

	// trace
	ctx, span := tracerProvider.SpanCtx(ctx, "service.ProducerEventKafka")
	defer span.End()

	trace_id := fmt.Sprintf("%v",ctx.Value("trace-request-id"))

	// create a mutex to avoid commit over a transaction on air
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Create a transacrion
	err = s.workerEvent.WorkerKafka.BeginTransaction()
	if err != nil {
		childLogger.Error().Interface("trace-request-id", trace_id ).Err(err).Msg("failed to kafka BeginTransaction")
		// Create a new producer and start a transaction
		err = s.workerEvent.DestroyWorkerEventProducerTx(ctx)
		if err != nil {
			return  err
		}
		s.workerEvent.WorkerKafka.BeginTransaction()
		if err != nil {
			return err
		}
		childLogger.Info().Interface("trace-request-id", trace_id ).Msg("success to recreate a new producer")
	}

	// prepare header
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, &carrier)
	
	headers_msk := make(map[string]string)
	for k, v := range carrier {
		headers_msk[k] = v
	}

	spanContext := span.SpanContext()
	headers_msk["trace-request-id"] = trace_id
	headers_msk["TraceID"] = spanContext.TraceID().String()
	headers_msk["SpanID"] = spanContext.SpanID().String()

	// publish event
	err = s.workerEvent.WorkerKafka.Producer(topic, keyMessage, &headers_msk, payloadBytes)

	//force a error SIMULATION
	if(trace_id == "force-rollback"){
		err = erro.ErrForceRollback
	}

	if err != nil {
		childLogger.Err(err).Interface("trace-request-id", trace_id ).Msg("KAFKA ROLLBACK !!!")
		err_msk := s.workerEvent.WorkerKafka.AbortTransaction(ctx)
		if err_msk != nil {
			childLogger.Err(err_msk).Interface("trace-request-id", trace_id ).Msg("failed to kafka AbortTransaction")
			return err_msk
		}
		return err
	}

	err = s.workerEvent.WorkerKafka.CommitTransaction(ctx)
	if err != nil {
		childLogger.Err(err).Interface("trace-request-id", trace_id ).Msg("Failed to Kafka CommitTransaction = KAFKA ROLLBACK COMMIT !!!")
		err_msk := s.workerEvent.WorkerKafka.AbortTransaction(ctx)
		if err_msk != nil {
			childLogger.Err(err_msk).Interface("trace-request-id", trace_id ).Msg("failed to kafka AbortTransaction during CommitTransaction")
			return err_msk
		}
		return err
	}

	childLogger.Info().Interface("trace-request-id", trace_id ).Msg("KAFKA COMMIT !!!")	

	return
}

//About Payment 
func (s * WorkerService) GetPayment(ctx context.Context, payment model.Payment) (*[]model.Payment, error){
	childLogger.Info().Str("func","GetPayment").Interface("trace-resquest-id", ctx.Value("trace-request-id")).Send()

	// get pix 
	res_list_payment, err := s.workerRepository.GetPayment(ctx, payment)
	if err != nil {
		return nil, err
	}

	return res_list_payment, nil
}

// About check health service
func (s * WorkerService) HealthCheck(ctx context.Context) error{
	childLogger.Info().Str("func","HealthCheck").Interface("trace-resquest-id", ctx.Value("trace-request-id")).Send()

	// Trace
	ctx, span := tracerProvider.SpanCtx(ctx, "service.HealthCheck")
	defer span.End()
	trace_id := fmt.Sprintf("%v",ctx.Value("trace-request-id"))

	// Check database health

	err := s.workerRepository.DatabasePGServer.Ping()
	if err != nil {
		log.Error().Err(err).Msg("*** Database health check FAILED ***")
		return erro.ErrHealthCheck
	}
	childLogger.Info().Str("func","HealthCheck").Msg("*** Database health check SUCCESSFULL ***")

	// Set headers
	headers := map[string]string{
		"Content-Type":  "application/json;charset=UTF-8",
		"X-Request-Id": trace_id,
		"Host": s.apiService[0].HostName,
	}

	// Set client http	
	httpClient := go_core_api.HttpClient {
		Url: 	s.apiService[0].Url + "/health",
		Method: s.apiService[0].Method,
		Timeout: s.apiService[0].HttpTimeout,
		Headers: &headers,
	}

	// get account_if from id (PK)
	_, _, err = apiService.CallRestApiV1(	ctx,
											s.goCoreRestApiService.Client,
											httpClient, 
											nil)
	if err != nil {
		log.Error().Err(err).Msg("*** Service ACCOUNT HEALTH FAILED ***")
		return erro.ErrHealthCheck
	}
	childLogger.Info().Str("func","HealthCheck").Msg("*** Service ACCOUNT HEALTH SUCCESSFULL ***")

	// Set headers
	headers = map[string]string{
		"Content-Type":  "application/json;charset=UTF-8",
		"X-Request-Id": trace_id,
		"Host": s.apiService[1].HostName,
	}

	// Set client http	
	httpClient = go_core_api.HttpClient {
		Url: 	s.apiService[1].Url + "/health",
		Method: s.apiService[1].Method,
		Timeout: s.apiService[1].HttpTimeout,
		Headers: &headers,
	}

	// get account_if from id (PK)
	_, _, err = apiService.CallRestApiV1(	ctx,
											s.goCoreRestApiService.Client,
											httpClient, 
											nil)
	if err != nil {
		log.Error().Err(err).Msg("*** Service LIMIT HEALTH FAILED ***")
		return erro.ErrHealthCheck
	}
	childLogger.Info().Str("func","HealthCheck").Msg("*** Service LIMIT HEALTH SUCCESSFULL ***")

	
	// Set headers
	headers = map[string]string{
		"Content-Type":  "application/json;charset=UTF-8",
		"X-Request-Id": trace_id,
		"Host": s.apiService[2].HostName,
	}

	// Set client http	
	httpClient = go_core_api.HttpClient {
		Url: 	s.apiService[2].Url + "/health",
		Method: s.apiService[2].Method,
		Timeout: s.apiService[2].HttpTimeout,
		Headers: &headers,
	}

	// get account_if from id (PK)
	_, _, err = apiService.CallRestApiV1(	ctx,
											s.goCoreRestApiService.Client,
											httpClient, 
											nil)
	if err != nil {
		log.Error().Err(err).Msg("*** Service LEDGER HEALTH FAILED ***")
		return erro.ErrHealthCheck
	}
	childLogger.Info().Str("func","HealthCheck").Msg("*** Service LEDGER HEALTH SUCCESSFULL ***")
	
	// Set headers
	headers = map[string]string{
		"Content-Type":  "application/json;charset=UTF-8",
		"X-Request-Id": trace_id,
		"Host": s.apiService[2].HostName,
	}

	// Set client http	
	httpClient = go_core_api.HttpClient {
		Url: 	s.apiService[2].Url + "/health",
		Method: s.apiService[2].Method,
		Timeout: s.apiService[2].HttpTimeout,
		Headers: &headers,
	}

	// get account_if from id (PK)
	_, _, err = apiService.CallRestApiV1(	ctx,
											s.goCoreRestApiService.Client,
											httpClient, 
											nil)
	if err != nil {
		log.Error().Err(err).Msg("*** Service CARD HEALTH FAILED ***")
		return erro.ErrHealthCheck
	}
	childLogger.Info().Str("func","HealthCheck").Msg("*** Service CARD HEALTH SUCCESSFULL ***")

	return nil
}