# Plano de implementação e critérios de conclusão

Design entregue nesta conversa antes do código: `design.md` e PDF. O envio por e-mail aos avaliadores continua a cargo do candidato.

## Etapas
1. Registrar skill, critérios e design em commit.
2. Implementar monólito Go, schema/migrações, transação financeira, saldo/extrato e outbox com simulador idempotente.
3. Implementar unitários e aceitação Gherkin, com banco real e concorrência.
4. Automatizar os gates: coverage >=85% interno, 100% ledger, mutantes financeiros 100%, complexidade <=15, tamanho e dependências.
5. Executar QA com race detector; corrigir falhas sem enfraquecer invariantes.
6. Subir duas APIs via Compose, simular morte da aplicação/indisponibilidade/reenvio, executar carga e auditor independente.
7. Medir 2.000 req/s em hot account, documentar resultados e qualquer limite. Atualizar README, decisões e histórico de IA.

## Mapeamento das dez regras
| Regra | Evidência |
|---|---|
| Unit tests | `go test`, testes de validação/limites/decisão |
| Aceitação/Gherkin | Godog com HTTP e PostgreSQL |
| Coverage | profiles Go e relatório por pacote |
| Mutation testing | mutantes semânticos selecionados no núcleo financeiro |
| Complexidade ciclomática | análise AST das funções |
| Tamanho dos módulos | limites de linhas por arquivo/pacote |
| Dependências | grafo de imports real e regras de camadas |
| Métricas de qualidade | relatório de QA e benchmark, com ambiente |
| Procedimentos de QA | Makefile + pipeline CI + roteiro de falhas |
| Testes finais | Compose, aceitação, falhas, carga e auditoria |

Configurações de teste usam banco descartável separado. Os cenários destrutivos nunca rodam em uma URL arbitrária de produção. Não há publicação automática de repositório nem envio de e-mail.
