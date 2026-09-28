---
name: pix-quality
description: Implementar e revisar este autorizador Pix com testes financeiros, aceitação Gherkin, cobertura, mutação e gates de qualidade. Use ao alterar código neste repositório.
---

# Qualidade do autorizador Pix

Use `docs/implementation-plan.md` como checklist. Antes de concluir, execute `make qa` e os testes finais documentados no README. Registre falhas e limitações em `reports/`, sem reduzir limites para esconder regressões.

- Testes unitários devem cobrir limites monetários, validação e decisões, incluindo valor exato do saldo e overflow.
- Cenários `.feature` devem ser executados contra HTTP e PostgreSQL reais; documentação Gherkin isolada não basta.
- Cobertura do código interno: >= 85%; domínio financeiro: 100% de statements. Reportar limites do que essa métrica mede.
- Mutation testing: eliminar 100% dos mutantes financeiros selecionados; distinguir sobreviventes, erros de compilação e falhas de infraestrutura. A suíte selecionada não representa todos os mutantes possíveis.
- Complexidade ciclomática máxima 15 por função; arquivos de produção <= 350 linhas e pacotes internos <= 1.200 linhas. Dividir por responsabilidade, sem criar abstrações só para satisfazer números.
- Dependências: ledger usa apenas biblioteca padrão; postgres depende de ledger; HTTP depende de ledger/postgres; nenhum ciclo. Validar via `go list`, não só por desenho.
- Executar gofmt, go vet, race detector, testes, cobertura, mutação e verificação estrutural. Guardar relatórios com data e commit.
- Usar PostgreSQL real nos testes de concorrência, idempotência, falhas e auditoria. Nenhuma conta negativa, duas partidas por aprovação, nenhum efeito por negativa, um efeito por ID.
- Teste final: subir Compose, verificar readiness, executar aceitação, falhas e carga, reconciliar operações, auditar saldo e histórico. Reportar p99 e erros juntos; a meta de 2.000 req/s só é atendida se medida.
- Timeouts de commit são indeterminados; nunca convertê-los em negativas financeiras. Estorno externo exige rejeição definitiva e é atômico com o estado final.
