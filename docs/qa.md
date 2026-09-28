# Procedimento de QA

## Preparação e isolamento

`docker compose up -d --build` inicia PostgreSQL, duas APIs e simulador. `pix_test` é descartável: os testes truncam somente esse banco e recusam URLs fora do nome esperado. Não apontar testes para dados reais. A carga e a auditoria usam `pix`; o simulador usa `pix_external`. Testes de integração não executam em paralelo entre pacotes (`-p 1`).

## Gates automáticos

| Requisito | Comando / evidência |
|---|---|
| Unitários | `make unit`: limites de centavos, saldo exato, insuficiência, overflow e validação |
| Aceitação/Gherkin | `make acceptance`: feature em português com HTTP e PostgreSQL reais |
| Cobertura | `make coverage`: mínimo 85% de statements internos, 100% no domínio; HTML e JSON |
| Mutação | `make mutation`: nove mudanças semânticas compiláveis nas regras monetárias, todas devem ser mortas |
| Complexidade | `make structure`: máximo 15 por função interna de produção, AST Go |
| Módulos | Mesmo comando: máximo 350 linhas por arquivo e 1.200 por pacote interno |
| Dependências | `go list` rejeita ciclos; AST impede domínio de importar bibliotecas externas e persistência de importar HTTP |
| Métricas | JSON em reports: cobertura, mutação, estrutura, latências, vazão e auditoria |
| QA | `make qa`: formatação, vet, race detector, testes, cobertura, mutação e estrutura |
| Aplicação final | k6 + `python3 scripts/faults.py` + `make audit`, com reconciliação dos IDs |

O workflow GitHub Actions executa `make qa` com PostgreSQL real e publica os relatórios mesmo em caso de falha. Testes que dependem de banco são explicitamente pulados se executados sem TEST_DATABASE_URL; por isso o comando de validação é `make qa`, que configura essa variável.

## Carga e falhas

1. `make seed` cria um milhão de clientes, sem substituir saldos existentes. O crédito inicial é fixture explícita, não endpoint público.
2. Aquecer; executar k6 com identificador novo por execução, taxa de chegada aberta e fundos suficientes. Não usar executor de usuários constantes para alegar capacidade de chegada.
3. Repetir para entrada, saída e mistura, registrando ambiente, duração, p99, erros e descartes. Resultados curtos são diagnóstico, não substituem ensaio sustentado.
4. O teste de falhas derruba processos **deste Compose**: interrompe uma API por SIGKILL, pausa e reinicia o banco, e descarta uma resposta já confirmada. Não apaga volumes. Não executar em ambiente compartilhado de produção.
5. Repetir todos os IDs com resultado desconhecido; confrontar manifesto com operações e partidas. Uma resposta 503 nunca vale como DECLINED.
6. Auditar o histórico em snapshot consistente: saldos iniciais + partidas, todo prefixo, sequências, conservação e quantidade/valor/destinatários das partidas. Limites do auditor: verifica o banco observado; o manifesto é necessário para detectar uma solicitação totalmente ausente.

## Critério de entrega

Não basta p99 baixo: a execução saudável precisa de zero erros técnicos, zero descartes, decisões corretas e auditoria aprovada. Resultados de falha são reportados separadamente. Revisar arquivos de documentação e manter resultados negativos relevantes. Mudanças posteriores no caminho financeiro exigem nova validação.

## Limitações

Cobertura de statements não mede todos os caminhos. Nove mutantes selecionados não equivalem a mutação exaustiva, especialmente do SQL. Os testes são evidência para as invariantes; a justificativa de concorrência depende também de atomicidade, locks, constraints e protocolo de escrita. Docker em um computador não demonstra HA, perda de disco nem integração Pix real.
