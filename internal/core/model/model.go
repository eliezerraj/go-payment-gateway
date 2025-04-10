package model

import (
	"time"
	go_core_pg "github.com/eliezerraj/go-core/database/pg"
	go_core_observ "github.com/eliezerraj/go-core/observability"
	go_core_event "github.com/eliezerraj/go-core/event/kafka" 
)

type AppServer struct {
	InfoPod 		*InfoPod 					`json:"info_pod"`
	Server     		*Server     				`json:"server"`
	ConfigOTEL		*go_core_observ.ConfigOTEL	`json:"otel_config"`
	DatabaseConfig	*go_core_pg.DatabaseConfig  `json:"database"`
	ApiService 		[]ApiService	`json:"api_endpoints"`
	KafkaConfigurations	*go_core_event.KafkaConfigurations  `json:"kafka_configurations"`
	Topics 			[]string					`json:"topics"`		
}

type InfoPod struct {
	PodName				string 	`json:"pod_name"`
	ApiVersion			string 	`json:"version"`
	OSPID				string 	`json:"os_pid"`
	IPAddress			string 	`json:"ip_address"`
	AvailabilityZone 	string 	`json:"availabilityZone"`
	IsAZ				bool   	`json:"is_az"`
	Env					string `json:"enviroment,omitempty"`
	AccountID			string `json:"account_id,omitempty"`
}

type Server struct {
	Port 			int `json:"port"`
	ReadTimeout		int `json:"readTimeout"`
	WriteTimeout	int `json:"writeTimeout"`
	IdleTimeout		int `json:"idleTimeout"`
	CtxTimeout		int `json:"ctxTimeout"`
}

type ApiService struct {
	Name			string `json:"name_service"`
	Url				string `json:"url"`
	Method			string `json:"method"`
	XApigwApiId		string `json:"x-apigw-api-id"`
	HostName		string `json:"host_name"`
}

type MessageRouter struct {
	Message			string `json:"message"`
}

type Card struct {
	ID				int			`json:"id,omitempty"`
	FkAccountID		int			`json:"fk_account_id,omitempty"`
	AccountID		string		`json:"account_id,omitempty"`	
	CardNumber		string  	`json:"card_number,omitempty"`
	TokenData		string  	`json:"token_data,omitempty"`
	Holder			string  	`json:"holder,omitempty"`
	Type			string  	`json:"type,omitempty"`
	Model			string  	`json:"model,omitempty"`
	Atc				int			`json:"atc,omitempty"`
	Status			string  	`json:"status,omitempty"`
	ExpiredAt		time.Time 	`json:"expired_at,omitempty"`
	CreatedAt		time.Time 	`json:"created_at,omitempty"`
	UpdatedAt		*time.Time 	`json:"updated_at,omitempty"`
	TenantID		string  	`json:"tenant_id,omitempty"`
}

type Payment struct {
	ID				int			`json:"id,omitempty"`
	FkCardId		int			`json:"fk_card_id,omitempty"`
	CardNumber		string  	`json:"card_number,omitempty"`
	TokenData		string  	`json:"token_data,omitempty"`
	CardType		string  	`json:"card_type,omitempty"`
	CardModel		string  	`json:"card_model,omitempty"`
	CardAtc			int			`json:"card_atc,omitempty"`
	Mcc				string  	`json:"mcc,omitempty"`
	Status			string  	`json:"status,omitempty"`
	Currency		string  	`json:"currency,omitempty"`
	Amount			float64 	`json:"amount,omitempty"`
	PaymentAt		time.Time	`json:"payment_at,omitempty"`
	TransactionId	*string  	`json:"transaction_id,omitempty"`
	RequestId		*string  	`json:"request_id,omitempty"`
	FkTerminalId	int			`json:"fk_terminal_id,omitempty"`
	Terminal		string		`json:"terminal,omitempty"`
	StepProcess		*[]StepProcess	`json:"step_process,omitempty"`
	CreatedAt		time.Time 	`json:"created_at,omitempty"`
	UpdatedAt		*time.Time 	`json:"updated_at,omitempty"`
	TenantID		string  	`json:"tenant_id,omitempty"`
}

type StepProcess struct {
	Name		string  	`json:"step_process,omitempty"`
	ProcessedAt	time.Time 	`json:"processed_at,omitempty"`
}

type Terminal struct {
	ID				int			`json:"id,omitempty"`
	Name			string		`json:"name,omitempty"`
	CoordX			float64  	`json:"coord_x,omitempty"`
	CoordY			float64  	`json:"coord_y,omitempty"`
	Status			string  	`json:"status,omitempty"`
	CreatedAt		time.Time 	`json:"created_at,omitempty"`
	UpdatedAt		*time.Time 	`json:"updated_at,omitempty"`
}

type TransactionLimit struct {
	ID				int			`json:"id,omitempty"`
	Category		string 		`json:"category,omitempty"`
	CardNumber		string 		`json:"card_number,omitempty"`
	TransactionId	string 		`json:"transaction_id,omitempty"`
	Mcc				string 		`json:"mcc,omitempty"`
	Status			string 		`json:"status,omitempty"`
	TransactionAt	time.Time 	`json:"transaction_at,omitempty"`		
	Currency		string 		`json:"currency,omitempty"`
	Amount			float64 	`json:"amount,omitempty"`
	SumAmount		float64 	`json:"transaction_sum_amount,omitempty"`
	SumCount		int 		`json:"transaction_sum_count,omitempty"`
	TenantID		string 		`json:"tenant_id,omitempty"`
}

// go-ledger
type Moviment struct {
	AccountID		string  	`json:"account_id,omitempty"`
	Type			string  	`json:"type,omitempty"`
	Currency		string  	`json:"currency,omitempty"`
	Amount			float64		`json:"amount,omitempty"`
	TransactionAt	*time.Time 	`json:"transaction_at,omitempty"`
	TenantID		string  	`json:"tenant_id,omitempty"`
}

type Account struct {
	ID				int			`json:"id,omitempty"`
	AccountID		string		`json:"account_id,omitempty"`
	PersonID		string  	`json:"person_id,omitempty"`
	CreatedAt		time.Time 	`json:"created_at,omitempty"`
	UpdatedAt		*time.Time 	`json:"updated_at,omitempty"`
	UserLastUpdate	*string  	`json:"user_last_update,omitempty"`
	TenantId		string  	`json:"tenant_id,omitempty"`
}

type PixTransaction struct {
	ID				int			`json:"id,omitempty"`
	TransactionId	string 		`json:"transaction_id,omitempty"`
	TransactionAt	time.Time 	`json:"transaction_at,omitempty"`	
	RequestId		string 		`json:"request_id,omitempty"`
	AccountFrom		Account		`json:"account_from,omitempty"`
	AccountTo		Account		`json:"account_to,omitempty"`
	Status			string  	`json:"status,omitempty"`
	Currency		string 		`json:"currency,omitempty"`
	Amount			float64 	`json:"amount,omitempty"`	
	StepProcess		*[]StepProcess	`json:"step_process,omitempty"`
	CreatedAt		time.Time 	`json:"created_at,omitempty"`
	UpdatedAt		*time.Time 	`json:"updated_at,omitempty"`
}