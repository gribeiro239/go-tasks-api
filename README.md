# Go Tasks API

Um projeto pequeno para quem já programa e quer conhecer Go construindo um backend HTTP. Usa somente a biblioteca padrão, sem dependências externas.

## Executar

Requer **Go 1.22 ou superior** e Git. Instale o Go seguindo [a documentação oficial](https://go.dev/doc/install). Depois, no terminal:

```powershell
git clone https://github.com/gribeiro239/go-tasks-api.git
cd go-tasks-api
go version
go run ./cmd/api
```

Se você já está na pasta do projeto, basta executar `go run ./cmd/api`.

Neste ambiente de estudo também foi preparado um Go local em `.tools/go`, que não é incluído no repositório. Caso use essa instalação, adicione-a ao PATH da sessão do PowerShell antes de executar os comandos:

```powershell
$env:PATH = "$PWD\.tools\go\bin;$env:PATH"
```

A API atende em `http://127.0.0.1:8080`. Use outro terminal para enviar as requisições abaixo. Encerre o servidor com **Ctrl+C**.

As tarefas são salvas em `data/tasks.json` na primeira alteração bem-sucedida. O caminho é relativo ao diretório onde você inicia o programa. Reiniciar a API preserva os dados.

Configuração opcional por variáveis de ambiente (não há carregamento automático de `.env`):

```powershell
$env:HTTP_ADDR = '127.0.0.1:18080'
$env:DATA_FILE = 'data/minhas-tarefas.json'
go run ./cmd/api
```

Se o Windows retornar `bind: Foi feita uma tentativa de acesso a um soquete de uma maneira que é proibida pelas permissões de acesso`, a porta padrão pode estar bloqueada ou reservada. Tente a configuração `HTTP_ADDR` acima e use `http://127.0.0.1:18080` nas requisições. A variável vale para a sessão atual do terminal. Em outro terminal, configure `$base` com a mesma porta.

## Experimentar no PowerShell

```powershell
$base = 'http://127.0.0.1:8080'
# Se iniciou com HTTP_ADDR=127.0.0.1:18080, use:
# $base = 'http://127.0.0.1:18080'

# Verificar o servidor
Invoke-RestMethod "$base/health"

# Criar: guarde a resposta para usar o ID gerado
$body = @{ title = 'Aprender structs e interfaces em Go' } | ConvertTo-Json
$task = Invoke-RestMethod "$base/tasks" -Method Post -ContentType 'application/json; charset=utf-8' -Body ([System.Text.Encoding]::UTF8.GetBytes($body))
$task

# Listar e consultar uma tarefa
Invoke-RestMethod "$base/tasks"
Invoke-RestMethod "$base/tasks/$($task.id)"

# Concluir
Invoke-RestMethod "$base/tasks/$($task.id)" -Method Patch -ContentType 'application/json' -Body '{"completed":true}'

# Filtrar por concluídas ou pendentes
Invoke-RestMethod "$base/tasks?completed=true"
Invoke-RestMethod "$base/tasks?completed=false"

# Renomear e reabrir (false é uma atualização válida)
Invoke-RestMethod "$base/tasks/$($task.id)" -Method Patch -ContentType 'application/json' -Body '{"title":"Praticar testes em Go","completed":false}'

# Excluir: resposta 204, sem corpo
Invoke-RestMethod "$base/tasks/$($task.id)" -Method Delete
```

## Contrato HTTP

| Método | Rota | Resultado |
|---|---|---|
| GET | `/health` | 200, `{"status":"ok"}` |
| GET | `/tasks` | 200, array de tarefas (vazio: `[]`) |
| GET | `/tasks?completed=true` | 200, filtro por conclusão; também aceita `false` |
| POST | `/tasks` | 201, tarefa criada e cabeçalho `Location` |
| GET | `/tasks/{id}` | 200, tarefa encontrada |
| PATCH | `/tasks/{id}` | 200, tarefa atualizada |
| DELETE | `/tasks/{id}` | 204, sem corpo |

Corpo de criação: `{"title":"Estudar Go"}`. Na atualização, envie `title`, `completed` ou os dois. Campos nulos são tratados como ausentes; uma atualização precisa de ao menos um campo não nulo. IDs são inteiros positivos e não são reutilizados depois de excluir tarefas.

Exemplo de tarefa:

```json
{
  "id": 1,
  "title": "Estudar Go",
  "completed": false,
  "created_at": "2026-10-19T12:00:00Z",
  "updated_at": "2026-10-19T12:00:00Z"
}
```

O título é aparado nas pontas e deve ter de 1 a 200 caracteres Unicode. POST e PATCH exigem `Content-Type: application/json`, rejeitam campos desconhecidos e limitam o corpo a 1 MiB. Datas são geradas pelo servidor em UTC.

Erros dos handlers têm formato `{"error":"mensagem"}`: 400 para entrada inválida, 404 para tarefa inexistente, 413 para corpo excessivo, 415 para Content-Type incorreto e 500 para falha de armazenamento. Rotas e métodos sem handler usam as respostas padrão do `net/http` em texto (404/405).

## Como ler o projeto

```text
cmd/api/main.go               ponto de entrada, configuração e encerramento
internal/httpapi/api.go       rotas, leitura de JSON e respostas HTTP
internal/httpapi/api_test.go  testes do contrato HTTP com httptest
internal/task/store.go        modelo, concorrência e persistência
internal/task/store_test.go   testes de persistência e acessos concorrentes
go.mod                       nome do módulo e versão mínima do Go
```

Comece por `cmd/api/main.go`, siga para `httpapi.New` e acompanhe uma criação até `Store.Create`. O fluxo é **requisição → handler → store → arquivo JSON → resposta**.

Conceitos de Go presentes no código:

- `package` organiza código. `main` produz um executável; `internal` restringe a importação ao projeto pai.
- `struct` define os dados. Tags como `json:"title"` controlam sua representação JSON.
- Nomes com inicial maiúscula, como `Task`, são exportados. Nomes com minúscula são internos ao pacote.
- Métodos como `func (s *Store) Create(...)` associam comportamento ao tipo. O receptor com ponteiro permite trabalhar na mesma instância do armazenamento.
- Funções retornam resultados e erros explicitamente: `task, err := store.Create(...)`. `if err != nil` trata cada falha.
- `*bool` no PATCH distingue campo ausente (`nil`) de `false`. `&value` obtém um ponteiro e `*pointer` lê o valor apontado.
- `defer` agenda a liberação de recursos na saída da função, como o desbloqueio do mutex.
- `net/http` atende requisições concorrentemente. `sync.RWMutex` protege o estado compartilhado, permitindo leituras simultâneas e escrita exclusiva.
- `go func()` inicia uma goroutine para o servidor; um canal entrega o erro ao fluxo principal. `context` coordena o encerramento.
- `http.Handler` é uma interface da biblioteca padrão. A implementação usa `ServeMux`, com padrões de método e parâmetros de rota disponíveis desde Go 1.22.

## Verificar e compilar

```powershell
go fmt ./...
go test ./...
go vet ./...
New-Item -ItemType Directory -Force bin | Out-Null
go build -o bin/tarefas.exe ./cmd/api
.\bin\tarefas.exe
```

Opcional: `go test -race ./...` detecta condições de corrida; no Windows, requer CGO habilitado e um compilador C compatível instalado.

Os testes cobrem o ciclo HTTP completo, filtros, validações, persistência após reabrir o arquivo, IDs, criação concorrente e preservação do estado quando a escrita falha.

O workflow em `.github/workflows/ci.yml` verifica formatação, executa `go vet`, roda testes e compila o projeto em Linux e Windows a cada push e pull request. No Linux, também executa o detector de condições de corrida. A integração testa a versão mínima Go 1.22 e a versão estável atual. O resultado remoto aparece na aba **Actions** do repositório após publicar os commits.

## Escopo deste projeto

Este é um backend local de estudo, sem autenticação. Use **um processo por arquivo de dados**: o mutex protege goroutines dentro do mesmo processo, sem coordenar múltiplas instâncias. O JSON inteiro é regravado em cada alteração; não há paginação. O temporário reduz o risco de sobrescrever o arquivo parcialmente, mas não oferece as garantias de transação e recuperação de um banco de dados, especialmente em falhas de energia. Um arquivo inválido impede a inicialização para preservar os dados existentes.

Uma evolução natural é trocar a persistência por SQLite ou PostgreSQL e acrescentar autenticação depois de explorar esta versão.

Referências: [Tour of Go](https://go.dev/tour/), [net/http](https://pkg.go.dev/net/http), [testing](https://pkg.go.dev/testing).

## Licença

Este projeto está disponível sob a [licença MIT](LICENSE).
