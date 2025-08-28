package database

import (
	"context"
	"time"
	"errors"

	"github.com/go-payment-gateway/internal/core/model"
	"github.com/go-payment-gateway/internal/core/erro"

	go_core_observ "github.com/eliezerraj/go-core/observability"
	go_core_pg "github.com/eliezerraj/go-core/database/pg"

	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
)

var (
	tracerProvider go_core_observ.TracerProvider
	childLogger = log.With().Str("component","go-payment-gateway").Str("package","internal.adapter.database").Logger()
)

type WorkerRepository struct {
	DatabasePGServer *go_core_pg.DatabasePGServer
}

// Above new worker
func NewWorkerRepository(databasePGServer *go_core_pg.DatabasePGServer) *WorkerRepository{
	childLogger.Info().Str("func","NewWorkerRepository").Send()

	return &WorkerRepository{
		DatabasePGServer: databasePGServer,
	}
}

// Above get stats from database
func (w WorkerRepository) Stat(ctx context.Context) (go_core_pg.PoolStats){
	childLogger.Info().Str("func","Stat").Interface("trace-resquest-id", ctx.Value("trace-request-id")).Send()
	
	stats := w.DatabasePGServer.Stat()

	resPoolStats := go_core_pg.PoolStats{
		AcquireCount:         stats.AcquireCount(),
		AcquiredConns:        stats.AcquiredConns(),
		CanceledAcquireCount: stats.CanceledAcquireCount(),
		ConstructingConns:    stats.ConstructingConns(),
		EmptyAcquireCount:    stats.EmptyAcquireCount(),
		IdleConns:            stats.IdleConns(),
		MaxConns:             stats.MaxConns(),
		TotalConns:           stats.TotalConns(),
	}

	return resPoolStats
}

// About add payment
func (w *WorkerRepository) AddPayment(ctx context.Context, tx pgx.Tx, payment *model.Payment) (*model.Payment, error){
	childLogger.Info().Str("func","AddPayment").Interface("trace-resquest-id", ctx.Value("trace-request-id")).Send()

	// Trace
	span := tracerProvider.Span(ctx, "database.AddPayment")
	defer span.End()

	// Prepare
	payment.CreatedAt = time.Now()
	if payment.PaymentAt.IsZero(){
		payment.PaymentAt = payment.CreatedAt
	}

	// Query and execute
	query := `INSERT INTO payment ( fk_card_id, 
									card_number, 
									fk_terminal_id, 
									terminal, 
									card_type, 
									card_model, 
									payment_at, 
									mcc, 
									status, 
									currency, 
									amount,
									transaction_id,
									request_id,
									created_at,
									tenant_id)
				VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15) RETURNING id`

	row := tx.QueryRow(ctx, query,  payment.FkCardId,
									payment.CardNumber,
									payment.FkTerminalId,
									payment.Terminal,
									payment.CardType,
									payment.CardModel,
									payment.PaymentAt,
									payment.Mcc,
									payment.Status,
									payment.Currency,
									payment.Amount,
									payment.TransactionId,
									payment.RequestId,
									payment.CreatedAt ,
									payment.TenantID)

	var id int
	if err := row.Scan(&id); err != nil {
		return nil, errors.New(err.Error())
	}

	// set PK
	payment.ID = id

	return payment , nil
}

// About get terminal
func (w *WorkerRepository) GetTerminal(ctx context.Context, terminal model.Terminal) (*model.Terminal, error){
	childLogger.Info().Str("func","GetTerminal").Interface("trace-resquest-id", ctx.Value("trace-request-id")).Send()
	
	// Trace
	span := tracerProvider.Span(ctx, "database.GetTerminal")
	defer span.End()

	// Get connection
	conn, err := w.DatabasePGServer.Acquire(ctx)
	if err != nil {
		return nil, errors.New(err.Error())
	}
	defer w.DatabasePGServer.Release(conn)

	// prepare
	res_terminal := model.Terminal{}

	// query and execute
	query :=  `SELECT 	id, 
						name, 
						coord_x, 
						coord_y, 
						status, 
						created_at, 
						updated_at
				FROM terminal
				WHERE name =$1`

	rows, err := conn.Query(ctx, query, terminal.Name)
	if err != nil {
		return nil, errors.New(err.Error())
	}
	defer rows.Close()

	for rows.Next() {
		err := rows.Scan( 	&res_terminal.ID, 
							&res_terminal.Name, 
							&res_terminal.CoordX, 
							&res_terminal.CoordY, 
							&res_terminal.Status,
							&res_terminal.CreatedAt,
							&res_terminal.UpdatedAt,
		)
		if err != nil {
			return nil, errors.New(err.Error())
        }
		return &res_terminal, nil
	}
	
	return nil, erro.ErrNotFound
}

// About update payment
func (w *WorkerRepository) UpdatePayment(ctx context.Context, tx pgx.Tx, payment model.Payment) (int64, error){
	childLogger.Info().Str("func","UpdatePayment").Interface("trace-resquest-id", ctx.Value("trace-request-id")).Send()

	// Trace
	span := tracerProvider.Span(ctx, "database.UpdatePayment")
	defer span.End()

	// Query and execute
	query := `UPDATE payment
				set status = $2,
					updated_at = $3
				where id = $1`

	row, err := tx.Exec(ctx, query,	payment.ID,
									payment.Status,
									time.Now())
	if err != nil {
		return 0, errors.New(err.Error())
	}
	return row.RowsAffected(), nil
}
//--------------------

// About add pix transaction
func (w *WorkerRepository) AddPixTransaction(ctx context.Context, tx pgx.Tx, pixTransaction model.PixTransaction) (*model.PixTransaction, error){
	childLogger.Info().Str("func","AddPixTransaction").Interface("trace-resquest-id", ctx.Value("trace-request-id")).Send()

	// Trace
	span := tracerProvider.Span(ctx, "database.AddPixTransaction")
	defer span.End()

	// Prepare
	if pixTransaction.CreatedAt.IsZero(){
		pixTransaction.CreatedAt = time.Now()
	}

	// Query and execute
	query := `INSERT INTO pix_transaction ( fk_account_id_from, 
											account_id_from, 
											fk_account_id_to, 
											account_id_to, 
											transaction_id, 
											transaction_at, 
											currency, 
											amount, 
											status, 
											request_id,
											created_at)
				VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id`

	row := tx.QueryRow(ctx, query,  pixTransaction.AccountFrom.ID,
									pixTransaction.AccountFrom.AccountID,
									pixTransaction.AccountTo.ID,
									pixTransaction.AccountTo.AccountID,
									pixTransaction.TransactionId,
									pixTransaction.TransactionAt,		
									pixTransaction.Currency,
									pixTransaction.Amount,
									pixTransaction.Status,
									pixTransaction.RequestId,
									pixTransaction.CreatedAt ,
	)

	var id int
	if err := row.Scan(&id); err != nil {
		return nil, errors.New(err.Error())
	}

	// set PK
	pixTransaction.ID = id

	return &pixTransaction , nil
}

// About update pix_transaction
func (w *WorkerRepository) UpdatePixTransaction(ctx context.Context, tx pgx.Tx, pixTransaction model.PixTransaction) (int64, error){
	childLogger.Info().Str("func","UpdatePixTransaction").Interface("trace-resquest-id", ctx.Value("trace-request-id")).Send()

	// Trace
	span := tracerProvider.Span(ctx, "database.UpdatePixTransaction")
	defer span.End()

	// Query and execute
	query := `UPDATE pix_transaction
				SET status = $2,
					updated_at = $3
				WHERE id = $1`

	row, err := tx.Exec(ctx, query,	pixTransaction.ID,
									pixTransaction.Status,
									time.Now())
	if err != nil {
		return 0, errors.New(err.Error())
	}
	return row.RowsAffected(), nil
}

// About stat pix transaction
func (w *WorkerRepository) StatPixTransaction(ctx context.Context, pixStatusAccount model.PixStatusAccount) (*model.PixStatus, error){
	childLogger.Info().Str("func","StatPixTransaction").Interface("trace-resquest-id", ctx.Value("trace-request-id")).Send()

	// Trace
	span := tracerProvider.Span(ctx, "database.StatPixTransaction")
	defer span.End()

	// Get connection
	conn, err := w.DatabasePGServer.Acquire(ctx)
	if err != nil {
		return nil, errors.New(err.Error())
	}
	defer w.DatabasePGServer.Release(conn)

	// prepare
	res_listPixStatusCount := []model.PixStatusCount{}
	res_pixStatusCount := model.PixStatusCount{}

	// query and execute
	query1 :=  `select pt.status, count(*) as count
				from pix_transaction pt
				group by pt.status`

	rows1, err := conn.Query(ctx, query1)
	if err != nil {
		return nil, errors.New(err.Error())
	}
	defer rows1.Close()

	for rows1.Next() {
		err := rows1.Scan( 	&res_pixStatusCount.Status, 
							&res_pixStatusCount.Count, 
		)
		if err != nil {
			return nil, errors.New(err.Error())
        }
		res_listPixStatusCount = append(res_listPixStatusCount, res_pixStatusCount)
	}	

	// prepare
	res_pixStatusAccount:= model.PixStatusAccount{}

	// query and execute
	query2 :=  `select account_id_from, account_id_to , status, count(*)
				from pix_transaction pt
				where account_id_from = $1
				group by account_id_from, account_id_to, status
				order by account_id_from`

	rows2, err := conn.Query(ctx, query2, pixStatusAccount.AccountFrom)
	if err != nil {
		return nil, errors.New(err.Error())
	}
	defer rows2.Close()

	for rows2.Next() {
		err := rows2.Scan( 	&res_pixStatusAccount.AccountFrom, 
							&res_pixStatusAccount.AccountTo,
							&res_pixStatusAccount.Status, 
							&res_pixStatusAccount.Count,  
		)
		if err != nil {
			return nil, errors.New(err.Error())
        }
	}	

	res_pixStatus:= model.PixStatus{PixStatusCount: 	&res_listPixStatusCount,
									PixStatusAccount:	&res_pixStatusAccount,
									}

	return &res_pixStatus , nil
}

// About get pix_transaction
func (w *WorkerRepository) GetPixTransaction(ctx context.Context, pixTransaction model.PixTransaction) (*model.PixTransaction, error){
	childLogger.Info().Str("func","GetPixTransaction").Interface("trace-resquest-id", ctx.Value("trace-request-id")).Send()
	
	// Trace
	span := tracerProvider.Span(ctx, "database.GetPixTransaction")
	defer span.End()

	// Get connection
	conn, err := w.DatabasePGServer.Acquire(ctx)
	if err != nil {
		return nil, errors.New(err.Error())
	}
	defer w.DatabasePGServer.Release(conn)

	// prepare
	accountFrom := model.Account{}
	accountTo	:= model.Account{}
	res_pixTransaction := model.PixTransaction{}

	// query and execute
	query :=  `	SELECT id, 
						account_id_from, 
						account_id_to, 
						transaction_id, 
						transaction_at, 
						currency, 
						amount, 
						status, 
						request_id, 
						created_at, 
						updated_at
				FROM public.pix_transaction
				WHERE id = $1`

	rows, err := conn.Query(ctx, query, pixTransaction.ID)
	if err != nil {
		return nil, errors.New(err.Error())
	}
	defer rows.Close()

	for rows.Next() {
		err := rows.Scan( 	&res_pixTransaction.ID, 
							&accountFrom.AccountID, 
							&accountTo.AccountID, 
							&res_pixTransaction.TransactionId, 
							&res_pixTransaction.TransactionAt,
							&res_pixTransaction.Currency,
							&res_pixTransaction.Amount,
							&res_pixTransaction.Status,
							&res_pixTransaction.RequestId,
							&res_pixTransaction.CreatedAt,
							&res_pixTransaction.UpdatedAt,
		)
		res_pixTransaction.AccountFrom = accountFrom
		res_pixTransaction.AccountTo = accountTo
		if err != nil {
			return nil, errors.New(err.Error())
        }
		return &res_pixTransaction, nil
	}
	
	return nil, erro.ErrNotFound
}