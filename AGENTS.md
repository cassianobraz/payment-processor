Context: payment-processor, sistema de pagamentos em Go com 4 microsservicos (gateway, ledger, fraud, investigator), Clean Architecture por servico (domain/usecase/infra/adapter), gRPC entre servicos, Kafka para eventos assincronos.

Estilo de código:
- Comentários só qunado o porquê não é óbvio pelo nome ou pela estrutura. Nunca descreva o que o código já deixa claro. Nunca escreva bloco de doc de múltiplos parágrafos, uma ou duas linhas bastam.
- Sem abastrações prematuras: três linhas parecidas são melhores que uma abstração genéricas cedo demais. Só generalize quando o segundo uso real aparecer.
- Erros de negóvio são sentinelas (erros.New), comparados com errors.Is, nunca com == depois de qualquer wrapping. Erros de infraestrutura ganham contexto com fmt.Errorf("...: %w", err) ao subir uma camada.
- Generics (type Foo[F any]) só quando o motor é genuinamente independente do domínio (uma maquina de estados, um cache, um batcher). Nunca como troca de interface{}, tentando evitar escrever o tipo concreto.
- Testes são table-driven por padrão. Um fake de teste implementa a interface mínima da porta, sem framework de mock.

Arquitetura
- domain/ não importa nada além da stdlib.
- usecase/ orquestra só contra interfaces (portas), nunca contra infraestrutura concreta.
- infra/ implementa as portas (Postgres, gRPC, kafka, HTTP externo)
- adapter/ traduz transporte (proto <=> dominio)
- pkg/ é compartilhado entre serviços e não conhece nada de pagamentos

Antes de considerar uma tarefa concluída 
- Rode go build ./... e go vet ./...
- Não adicione dependência nova sem justificar por que a stdlib não resolve.
