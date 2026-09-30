# Autorizador Pix

Monólito em Go que autoriza transferências em centavos de real. O PostgreSQL é a autoridade do saldo: uma aprovação grava duas partidas no mesmo `COMMIT`, uma recusa não grava nenhuma, e a mesma `transaction_id` não movimenta o saldo duas vezes.

## Iniciar

Pré-requisitos: Docker com Compose, Go 1.26, Python 3. Carga e falhas também precisam de k6.

```bash
docker compose up -d --build
curl -fsS http://127.0.0.1:8080/readyz
curl -fsS http://127.0.0.1:8081/readyz
```

Sobe o PostgreSQL 16 em `127.0.0.1:55432`, duas APIs (`8080` e `8081`) e o simulador de destino externo. A chave local é `local-development-key`. `fsync` e `synchronous_commit` ficam ligados.

O schema nasce vazio. Para um teste manual, crie poucas contas. A carga de 2.000 req/s usa um milhão:

```bash
go run ./cmd/pix -seed 1000
# carga: make seed
```

Transferência interna de 1 centavo da conta 1 para a 2:

```bash
curl -sS -X POST http://127.0.0.1:8080/transfers \
  -H "Authorization: Bearer local-development-key" \
  -H "Content-Type: application/json" \
  -d '{"transaction_id":"demo-1","source_id":1,"destination_id":2,"amount_minor":1}'
```

A resposta `APPROVED` ou `DECLINED` só sai depois do `COMMIT`. Timeout ou `503` não é recusa financeira: repita a mesma `transaction_id` ou consulte `GET /transfers/demo-1`.

## Testar

`make qa` exige o Postgres do Compose no ar. Ele formata, roda `go vet`, testes com detector de corrida, cobertura, mutação e o limite estrutural. Os testes que usam banco truncam só `pix_test`.

```bash
make unit          # regras de saldo, valor exato, estouro e validação
make acceptance    # features/authorization.feature em HTTP e PostgreSQL reais
make qa            # portão completo; relatórios em reports/
```

![Cobertura interna 92,2% acima do limite de 85%, domínio e mutantes em 100%](docs/figures/qualidade.svg)

`reports/structure.json` passou no mesmo dia: complexidade ≤ 15, arquivo ≤ 350 linhas, pacote interno ≤ 1.200.

Falha injetada e auditoria usam o banco `pix`, separado de `pix_test`:

```bash
python3 scripts/faults.py
make audit
```

## Carga com k6

O script é `scripts/load.js`. Ele manda `POST /transfers` em chegada aberta para `http://localhost:8080` e `http://localhost:8081`, com a chave `local-development-key`. Cada pedido move 1 centavo. No modo `mixed`, a conta 1 alterna entre origem e destino; o par é `2 + (n % 999998)`, então o seed de um milhão de contas precisa existir antes.

Suba o Compose, espere as duas APIs e semeie o banco `pix`:

```bash
docker compose up -d --build
curl -fsS http://127.0.0.1:8080/readyz
curl -fsS http://127.0.0.1:8081/readyz
make seed
```

Para medir de novo a partir de saldo e volume limpos, apague o volume do Compose antes de subir:

```bash
docker compose down -v
docker compose up -d --build
make seed
```

`make seed` cria 1.000.000 clientes e as 32 contas técnicas. Conta que já existe conserva o saldo.

O ensaio de 15 segundos só confirma que o stack responde. O que vale é o de 10 minutos:

```bash
k6 run -e DURATION=15s -e RATE=2000 -e MODE=mixed -e RUN_ID=warmup scripts/load.js
k6 run -e DURATION=10m -e RATE=2000 -e MODE=mixed -e RUN_ID=ensaio-10m scripts/load.js
```

`make load` repete o ensaio de 10 minutos a 2.000 req/s com os padrões do script. O resumo sai em `reports/load-<RUN_ID>.json`. Com `RUN_ID=ensaio-10m`, o arquivo é `reports/load-ensaio-10m.json`.

| Variável | Padrão | Efeito |
|---|---|---|
| `RATE` | `2000` | Iterações pedidas por segundo |
| `DURATION` | `10m` | Duração do k6 (`15s`, `5m`, `10m`) |
| `MODE` | `mixed` | `mixed`, `in` (tudo entra na conta 1) ou `out` (tudo sai dela) |
| `RUN_ID` | timestamp | Prefixo de cada `transaction_id` e nome do JSON |
| `URLS` | as duas APIs | Lista separada por vírgula |
| `API_KEY` | `local-development-key` | Bearer das requisições |
| `COLD_RATE` | `0` | Segunda cena, entre contas frias, se for maior que zero |

O k6 só fecha com código 0 quando os quatro limiares passam: p99 abaixo de 300 ms, nenhuma falha HTTP, todo check com `decision` `APPROVED`, nenhuma iteração descartada. Código 99 significa limiar estourado; o JSON ainda é gravado. HTTP 200 é aprovação gravada. HTTP 503 é erro técnico (fila cheia ou espera além de 1,5 s): repita a mesma `transaction_id`. Status ausente é timeout do cliente (3 s) ou conexão fechada.

## Arquitetura

Duas instâncias HTTP compartilham um PostgreSQL. O lote (até 64 pedidos, janela padrão de 15 ms) serializa a escrita da conta quente com `pg_advisory_xact_lock`. Destino externo debita a origem, credita uma de 32 contas técnicas e deixa a entrega na outbox; o worker fala com o simulador fora da transação financeira.

![Cliente nas duas APIs, ambas gravam no PostgreSQL, e a outbox segue para o simulador](docs/figures/arquitetura.svg)

## Design do sistema

Quatro camadas. Quem está acima depende de quem está abaixo. O domínio só usa a biblioteca padrão e os próprios pacotes.

![Apresentação depende de aplicação e infraestrutura; as duas dependem do domínio](docs/figures/camadas.svg)

## Fluxo do cliente ao banco

Pedido misto na conta quente. Contas independentes não disputam a mesma linha; créditos e débitos da conta 1 disputam o lote.

![Cinco passos do POST até a decisão, com APPROVED em duas partidas e DECLINED sem partidas](docs/figures/fluxo.svg)

## Última execução

Arquivo `reports/load-camadas-reset-10m.json`, 30 set 2026, cerca de 16:21. k6 em chegada aberta, 2.000 req/s, 10 minutos, modo `mixed` nas portas 8080 e 8081.

![p99 de 1.548 ms nos 10 minutos, acima da linha de 300 ms; o ensaio de 5 minutos ficou em 154 ms](docs/figures/latencia.svg)

![Oferta de 2.000 req/s, 1.972 concluídos e 1.949 aprovados](docs/figures/vazao.svg)

![13.176 respostas 503, 3.083 iterações descartadas e 829 outros status](docs/figures/erros.svg)

Os quatro limiares do k6 falharam: p99, erro HTTP, checks e iterações descartadas. A meta de 2.000 req/s com p99 abaixo de 300 ms não foi atingida neste ensaio de 10 minutos. O ensaio de 5 minutos, no mesmo cenário, passou e está no gráfico de latência em verde. Ele não substitui os 10 minutos.
