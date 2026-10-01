# Autorizador Pix

Monólito em Go que autoriza transferências em centavos de real. O TigerBeetle 0.17.9 é a autoridade do saldo. Uma aprovação é uma transferência de partidas dobradas gravada num único commit, e uma recusa não move saldo. A mesma `transaction_id` não movimenta o saldo duas vezes.

## Iniciar

Pré-requisitos: Docker com Compose, Go 1.26 com cgo (o cliente do TigerBeetle é uma biblioteca C) e Python 3. Carga e falhas também precisam de k6.

```bash
docker compose up -d --build --wait
curl -fsS http://127.0.0.1:8080/readyz
curl -fsS http://127.0.0.1:8081/readyz
```

Sobe:

- o TigerBeetle em `127.0.0.1:3000`, com volume `tbdata` e cache de 2 GiB (usa cerca de 4 GiB de RAM);
- duas APIs (`8080` e `8081`);
- o simulador de destino externo.

A chave local é `local-development-key`. O TigerBeetle e as APIs precisam de `seccomp=unconfined` por causa do io_uring; o Compose já configura isso.

Na primeira subida, as contas técnicas, as contas de sistema e a emissão fixa de `MaxInt64` centavos no tesouro são criadas sozinhas. Para um teste manual, crie poucas contas. A carga de 2.000 req/s usa um milhão:

```bash
go run ./cmd/pix -seed 1000
# carga: make seed
```

Transferência interna de 1 centavo da conta 1 para a 2. A `transaction_id` é um UUID (RFC 4122, versão 1 a 8):

```bash
curl -sS -X POST http://127.0.0.1:8080/transfers \
  -H "Authorization: Bearer local-development-key" \
  -H "Content-Type: application/json" \
  -d '{"transaction_id":"0190f3a2-7c1e-4d2b-9a6f-1234567890ab","source_id":1,"destination_id":2,"amount_minor":1}'
```

A resposta `APPROVED` ou `DECLINED` só sai depois do commit no TigerBeetle. Timeout ou `503` não é recusa financeira: repita a mesma `transaction_id` ou consulte `GET /transfers/{id}`. Repetir o id com outro conteúdo devolve `409`.

## Testar

Os testes de integração usam um TigerBeetle descartável em modo `--development`, na porta `3001`, que só sobe com o perfil `test`. Cada teste usa contas e UUIDs aleatórios, porque o TigerBeetle não apaga dados.

```bash
make test-ledger   # sobe o cluster de teste
make unit          # regras de saldo, valor exato, saldo disponível e validação
make acceptance    # features/authorization.feature em HTTP e TigerBeetle reais
make qa            # portão completo; relatórios em reports/
```

`make qa` formata, roda `go vet`, testes com detector de corrida, cobertura, mutação e o limite estrutural. Na última execução (`reports/tigerbeetle-2026-10-01.md`):

- cobertura interna de 95,3% e do domínio de 100%;
- 12 de 12 mutantes financeiros mortos;
- nenhuma violação de complexidade, tamanho ou dependência.

No macOS, o Makefile põe `CGO_LDFLAGS=-Wl,-ld_classic`, porque o linker novo da Apple recusa o arquivo do cliente.

Falha injetada e auditoria usam o cluster principal. A auditoria precisa de tráfego parado, porque o TigerBeetle não tem snapshot de leitura:

```bash
python3 scripts/faults.py
make audit
```

## Carga com k6

O script é `scripts/load.js`. Ele manda `POST /transfers` em chegada aberta, com a chave `local-development-key`, e cada pedido move 1 centavo. No modo `mixed`, a conta 1 alterna entre origem e destino; o par é `2 + (n % 999998)`, então o seed de um milhão de contas precisa existir antes.

Para medir a partir de um ledger limpo:

```bash
docker compose down -v
docker compose up -d --build --wait
make seed      # 1.000.000 contas, cerca de 10 s
sleep 90       # deixa a compactação pós-seed assentar
```

O ensaio que passou rodou o k6 dentro da rede do Compose, falando direto com `api:8080` e `api2:8080`:

```bash
docker run --rm --network drac-pix_default -v "$PWD":/work -w /work \
  -e RATE=2000 -e DURATION=10m -e RUN_ID=ensaio-10m \
  -e URLS=http://api:8080,http://api2:8080 grafana/k6 run scripts/load.js
```

`make load` roda do host, passando pelo encaminhamento de portas do Docker Desktop (`localhost:8080,8081`). O resumo sai em `reports/load-<RUN_ID>.json`.

| Variável | Padrão | Efeito |
|---|---|---|
| `RATE` | `2000` | Iterações pedidas por segundo |
| `DURATION` | `10m` | Duração do k6 (`15s`, `5m`, `10m`) |
| `MODE` | `mixed` | `mixed`, `in` (tudo entra na conta 1) ou `out` (tudo sai dela) |
| `RUN_ID` | timestamp | Nome do JSON |
| `URLS` | as duas APIs no host | Lista separada por vírgula |
| `API_KEY` | `local-development-key` | Bearer das requisições |
| `COLD_RATE` | `0` | Segunda cena, entre contas frias, se for maior que zero |
| `TB_BATCH_WINDOW` (Compose) | `5ms` | Janela em que a API junta criações num só pedido ao TigerBeetle; `0` desliga |

O k6 só fecha com código 0 quando os quatro limiares passam: p99 abaixo de 300 ms, nenhuma falha HTTP, todo check com `decision` `APPROVED` e nenhuma iteração descartada. Código 99 significa limiar estourado; o JSON ainda é gravado. HTTP 503 é resultado desconhecido (espera além de 1,5 s): repita a mesma `transaction_id`. Status ausente é timeout do cliente (3 s) ou conexão fechada.

## Arquitetura

Duas instâncias HTTP compartilham um cluster TigerBeetle de uma réplica. Não há lote de correção, fila nem lock na aplicação: o TigerBeetle verifica saldo (`debits_must_not_exceed_credits`) e id no mesmo commit, e cada evento de um pedido tem resultado próprio. A API só junta as criações de 5 ms num pedido, por vazão. Sem isso, o cliente manda cerca de uma transferência por pedido, e o custo por pedido travava o ledger.

Destino externo grava, numa cadeia atômica, uma reserva (transferência pendente para uma de 32 contas técnicas) e a referência do destino em transferências-memo de valor zero. O worker varre as reservas sem liquidação, entrega ao simulador e lança `post` (SETTLED) ou `void` (REVERSED). O TigerBeetle resolve uma reserva no máximo uma vez, então o estorno não acontece duas vezes.

Decisões e limites estão em `docs/decisions.md`.

## Design do sistema

Quatro camadas. Quem está acima depende de quem está abaixo. O domínio só usa a biblioteca padrão e os próprios pacotes; só a infraestrutura importa o cliente do TigerBeetle.

![Apresentação depende de aplicação e infraestrutura; as duas dependem do domínio](docs/figures/camadas.svg)

## Última execução

Arquivo `reports/load-tb-10m-mixed-innet.json`, 1 out 2026: 2.000 req/s por 10 minutos, modo `mixed`, k6 na rede do Compose. Os quatro limiares passaram:

| Métrica | Valor |
|---|---|
| Requisições | 1.200.001 (2.000/s) |
| p50 / p99 / máx. | 5,3 / 74,4 / 994 ms |
| Falhas HTTP | 0 |
| Checks `APPROVED` | 100% |
| Iterações descartadas | 0 |

A mesma carga disparada do host deu p99 de 148 ms e nenhuma iteração descartada, mas teve 4 conexões fechadas (`EOF`) em 1,2 milhão. Por isso não passou. O ensaio que passou começou com o ledger limpo; num volume com uns 5 milhões de transferências, um teste de 60 s logo após reiniciar o TigerBeetle travou 10 s, e os 10 minutos não foram repetidos nesse volume.

Depois da carga e das falhas injetadas, a auditoria passou: 1.000.000 de contas, nenhum saldo inválido, dinheiro conservado e 2.520.827 linhas de histórico conferidas.

- PDF com gráficos de latência, vazão por minuto e requisições em voo, todos os ensaios, auditoria e falhas: [`output/pdf/ensaio-tigerbeetle-10m.pdf`](output/pdf/ensaio-tigerbeetle-10m.pdf). Para gerar de novo: `python3 reports/_build_tigerbeetle_pdf.py`, depois imprimir `reports/ensaio-tigerbeetle-10m.html` com o Chrome headless (`--print-to-pdf`).
- Relatório completo, inclusive as execuções que falharam antes da janela de 5 ms e do cache de 2 GiB: `reports/tigerbeetle-2026-10-01.md`.
- Os PDFs `ensaio-reset-10m.pdf` e `ensaio-camadas-10m.pdf` são do desenho anterior, com PostgreSQL, e ficam como histórico.
