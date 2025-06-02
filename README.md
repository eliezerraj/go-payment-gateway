# go-payment-gateway
    worklod for POC purposes

# usecase

    AddPayment
    create a payment (from a plain card)
        "step_process": 
        {
            "step_process": "AUTHORIZATION:STATUS:PENDING",
            "processed_at": "2025-06-02T00:51:21.569162547Z"
        },
        {
            "step_process": "LIMIT:BREACH_LIMIT:CREDIT",
            "processed_at": "2025-06-02T00:51:21.700409473Z"
        },
        {
            "step_process": "LEDGER:WITHDRAW:OK",
            "processed_at": "2025-06-02T00:51:21.919879071Z"
        },
        {
            "step_process": "CARD-ATC:OK",
            "processed_at": "2025-06-02T00:51:21.988950408Z"
        },
        {
            "step_process": "AUTHORIZATION:STATUS:OK",
            "processed_at": "2025-06-02T00:51:21.998375193Z"
        }

    PixTransaction
    create a pix_transaction (ledger update via event)
        "step_process":
            {
                "step_process": "ACCOUNT-FROM:OK",
                "processed_at": "2025-06-02T00:49:10.387729402Z"
            },
            {
                "step_process": "ACCOUNT-TO:OK",
                "processed_at": "2025-06-02T00:49:10.429096785Z"
            },
            {
                "step_process": "PIX-TRANSACTION:STATUS:PENDING",
                "processed_at": "2025-06-02T00:49:10.443832332Z"
            },
            {
                "step_process": "LEDGER:WIRE-TRANSFER:IN-QUEUE",
                "processed_at": "2025-06-02T00:49:10.510085761Z"
            },
            {
                "step_process": "PIX-TRANSACTION:STATUS:IN-QUEUE",
                "processed_at": "2025-06-02T00:49:10.522326747Z"
            }

    GetPixTransaction
    get all information about a pix
    

    StatPixTransaction
    how many pix transaction is queued (waiting for consume)

# tables

    table payment
    id    |fk_card_id|card_number    |fk_terminal_id|terminal|card_type|card_model|payment_at                   |mcc |status               |currency|amount|request_id                          |transaction_id                      |fraud|created_at                   |updated_at                   |
------+----------+---------------+--------------+--------+---------+----------+-----------------------------+----+---------------------+--------+------+------------------------------------+------------------------------------+-----+-----------------------------+-----------------------------+
   192|        35|111.111.111.500|             1|TERM-1  |CREDIT   |CHIP      |2025-04-29 16:11:59.289 -0300|FOOD|AUTHORIZATION-GRPC:OK|BRL     |  22.0|626b7d82-587a-4c9e-b1cf-b9cab2efbe9b|b30d8aea-32be-480e-8c1d-afe01f9dbdf3|     |2025-04-29 16:11:59.289 -0300|2025-04-29 16:12:00.347 -0300|
   194|        35|111.111.111.500|             1|TERM-1  |CREDIT   |CHIP      |2025-04-29 21:14:11.539 -0300|FOOD|AUTHORIZATION:OK     |BRL     |  11.0|c773ed08-b853-4bb7-b4ff-a7d5d23d4c60|f643a74a-6071-4462-8240-1acd6d4afbc5|     |2025-04-29 21:14:11.539 -0300|2025-04-29 21:14:12.079 -0300|
   198|       519|111.111.000.155|             1|TERM-1  |CREDIT   |CHIP      |2025-04-29 21:38:00.409 -0300|FOOD|AUTHORIZATION:OK     |BRL     | 124.0|4466f764-0044-495e-8a2d-6c1301de3a0d|e46d7ea0-4f0c-4dd4-8f55-bc30009a9baf|     |2025-04-29 21:38:00.409 -0300|2025-04-29 21:38:00.920 -0300|
   199|       595|111.111.000.231|             1|TERM-1  |CREDIT   |CHIP      |2025-04-29 21:38:44.476 -0300|FOOD|AUTHORIZATION:OK     |BRL     |  99.0|2958b570-a55c-4aa4-a158-fd6baf46626b|034b8b9c-d35b-4e55-a6b8-5f43a0d183ec|     |2025-04-29 21:38:44.476 -0300|2025-04-29 21:38:44.903 -0300|

    table pix_transaction
    id    |fk_account_id_from|account_id_from|fk_account_id_to|account_id_to|transaction_id                      |transaction_at               |currency|amount|status           |request_id|created_at                   |updated_at                   |
    ------+------------------+---------------+----------------+-------------+------------------------------------+-----------------------------+--------+------+-----------------+----------+-----------------------------+-----------------------------+
    115061|              1503|ACC-10000      |            1504|ACC-20000    |87de1c03-b9a5-4dfc-b79c-70f1efea4ab9|2025-05-14 18:25:53.949 -0300|BRL     |   1.0|IN-QUEUE:CONSUMED|          |2025-05-14 18:25:53.949 -0300|2025-05-14 23:40:06.434 -0300|