# Handoff: Google Chat Client

Atualizado em 19/10/2026. O usuário pediu para encerrar a sessão e registrar o trabalho para retomar depois.

## Estado ao encerrar

Existe uma primeira implementação de backend e interface, com dependências instaladas. **O aplicativo ainda não foi entregue como funcional: a verificação de tipos da interface está falhando, não houve execução da interface nem empacotamento final.**

Os quatro testes locais do pacote de integração passaram. Não houve autenticação com conta real, chamadas ao Google nem envio de mensagens a pessoas.

Próxima ação técnica: corrigir o tipo de `children` no componente `IconButton` em `src/App.tsx`, regenerar a ponte TypeScript e continuar as verificações. Não recomeçar a pesquisa ou a implementação do zero.

## Pedido e decisões do usuário

- Criar um aplicativo desktop para usar as conversas reais do Google Chat com login Google.
- Windows, Linux e macOS são os alvos.
- Chat é a prioridade. Calendar, Meet e Gmail são expansões futuras.
- **Múltiplas contas são essenciais**, incluindo contas corporativas de organizações diferentes.
- O usuário tem uma outra conta corporativa da qual é administrador e pode usá-la na validação enquanto solicita liberação no novo trabalho.
- Rocket.Chat foi apenas um exemplo. Não há preferência por um projeto específico; a prioridade é uma interface bonita e funcional e avançar sem gastar muito tempo.
- O usuário delegou a escolha da tecnologia, autorizando Tauri ou outra alternativa.
- Indicou [MyGo](https://mygo.egoist.dev/docs/getting-started), que foi investigado e escolhido.
- Aprovou aplicar a skill antislop **durante a construção**.
- Autorizou poucos subagentes, mais baratos que o agente principal. Foi usado somente um subagente `gpt-6-luna`, já concluído.
- A última instrução foi produzir este handoff e parar. Não continuar a implementação nesta sessão.

## Tecnologia escolhida

| Parte | Escolha atual |
| --- | --- |
| Aplicativo desktop | MyGo 0.5.5 |
| Backend | Go 1.27.1 |
| Interface | React 19.3.0, TypeScript 7.0.2 e Fluent UI 9.74.10 |
| Ícones | `@fluentui/react-icons` 2.0.344 |
| Ferramentas da interface | Vite 8.3.4, Biome 2.5.13 |
| OAuth | `golang.org/x/oauth2` 0.36.0 |
| Credenciais persistentes | `mygo.Secrets`, usando o chaveiro do sistema |
| Interface/backend | Cliente TypeScript gerado por MyGo a partir dos métodos Go |

MyGo usa webviews do sistema: WKWebView no macOS, WebKitGTK no Linux e WebView2 no Windows. A proposta é um executável desktop com interface React, não uma interface feita de widgets nativos Go. Essa distinção está registrada em `docs/design.md`.

O framework oferece APIs de janela, diálogos, abertura de navegador, notificações e credenciais. O ambiente já tinha Go e Bun compatíveis, GTK 3 e WebKitGTK 4.1. Não foi instalado SDK de Tauri/Electron.

A sugestão anterior de Electron + Fuselage foi substituída por MyGo + Fluent UI. Não foi clonado ou adaptado um servidor Rocket.Chat/Mattermost. O pacote `@fluentui-contrib/react-chat` foi pesquisado, mas **não é dependência desta implementação**; o frontend atual compõe os controles gerais do Fluent UI.

## Mapa do que foi escrito

| Arquivo | Responsabilidade |
| --- | --- |
| `main.go` | Inicialização MyGo, registro do serviço, janela e menus; abertura externa de navegação web |
| `desktop.go` | Métodos disponíveis à interface, importação de JSON OAuth por diálogo nativo, login cancelável e adaptação para `mygo.Secrets` |
| `internal/chat/types.go` | Contas, espaços, mensagens, configuração e interface `Vault` |
| `internal/chat/service.go` | Configuração local, seleção/remoção de contas, sessões por conta e renovação de tokens |
| `internal/chat/oauth.go` | Login Google com navegador do sistema, PKCE, callback de loopback, identidade e persistência da conta |
| `internal/chat/api.go` | Chamadas à API Google Chat, espaços, nomes de DMs/grupos, histórico, threads e envio |
| `internal/chat/service_test.go` | Quatro testes locais com Vault e transporte HTTP simulados |
| `src/App.tsx` | Interface completa inicial: contas, configuração, conversas, histórico, threads, editor e temas |
| `src/style.css` | Layout e temas claro/escuro, estados e adaptação a janelas estreitas |
| `src/main.tsx` | Entrada React |
| `src/mygo.ts` | Ponte gerada pelo MyGo; existe localmente e está no `.gitignore` |
| `mygo.config.ts` | Nome, identificador, versão, comandos Vite, caminho da ponte e saída dos pacotes |
| `vite.config.ts`, `tsconfig.json`, `index.html` | Configuração da interface, incluindo CSP |
| `Makefile` | Fachada dos comandos de desenvolvimento, geração, qualidade e build |
| `package.json`, `package-lock.json`, `go.mod`, `go.sum` | Dependências e versões |
| `docs/pesquisa-inicial.md` | Pesquisa anterior com comparações e capacidades da API; contém recomendações históricas que precisam ser contextualizadas |
| `docs/design.md` | Direção visual e razões das decisões |
| `README.md` | Ainda descreve a fase de pesquisa e a sugestão antiga; precisa ser atualizado |

Não foi criado `AGENTS.md` nem instalada outra skill no repositório.

## Backend implementado, ainda sem validação com Google

### Autenticação

- Importação do JSON baixado de um cliente OAuth do tipo **Aplicativo para computador**, esperando a chave `installed`.
- Client ID e client secret guardados em `mygo.Secrets`, chave `oauth-client`.
- Login no navegador externo com Authorization Code, PKCE S256, `state` aleatório e `access_type=offline`.
- O consentimento usa `prompt=consent select_account` para permitir escolher outra conta e obter refresh token.
- Listener apenas em `127.0.0.1`, porta aleatória e `/oauth/callback`.
- Validação de `state` com comparação constante, limite de três minutos e cancelamento pela interface.
- Consulta ao endpoint OpenID Connect `userinfo`, verificando email e usando `sub` como ID estável da conta.
- Refresh token individual em `refresh:<sub>`. Access tokens ficam em memória no backend.
- Tokens e client secret não são retornados pela ponte TypeScript.
- `accounts.json` guarda metadados de conta, ID do cliente e conta ativa, sem os tokens. A gravação usa arquivo temporário e rename, com permissões restritas.
- Mutex por sessão para serializar renovação/remoção de cada conta.

Escopos atuais:

```text
openid
email
profile
https://www.googleapis.com/auth/chat.spaces.readonly
https://www.googleapis.com/auth/chat.memberships.readonly
https://www.googleapis.com/auth/chat.messages
```

O escopo de memberships foi incluído para obter nomes dos participantes em DMs/grupos sem nome. Não foram incluídos escopos Calendar, Meet, Gmail ou estado de leitura.

### API Google Chat

- Espaços paginados, 100 por página.
- Resolução de nomes por memberships quando o espaço não fornece nome, com limite de quatro consultas paralelas e cache por conta.
- Histórico paginado, 50 mensagens por página, solicitado em ordem descendente e apresentado em ordem crescente.
- Filtro de histórico por thread e validação de que a thread pertence ao espaço informado.
- Envio de texto e resposta em thread.
- `requestId` reaproveitado nas tentativas do mesmo envio, para a idempotência oferecida pelo Google.
- Validação dos nomes de recursos e mensagens vazias/grandes.
- Erros de rede, permissão, sessão, cota e recurso indisponível apresentados em português.

### Interface escrita

- Faixa de contas à esquerda, com iniciais derivadas das contas reais.
- Seleção e remoção de contas, adição de outra conta e cancelamento do login.
- Conta/email visível na navegação e no editor.
- Configurações para importação OAuth e aparência.
- Lista de conversas, filtro local, categorias e paginação.
- Histórico e painel de thread, paginação de mensagens antigas, editor e tentativa de reenvio.
- Rascunhos em memória separados por conta, espaço e thread; não há persistência desses textos.
- Enter envia; Shift+Enter quebra linha; respeita composição de texto.
- Consulta automática da conversa aberta a cada 15 segundos, com recuo após erros até 120 segundos.
- Temas claro, escuro e seguindo o sistema.
- Estados de carregamento, vazio e erro.
- Prévia em navegador detecta a ausência do runtime MyGo e desabilita as ações de conexão.
- Nenhum dado de conversa fictício foi adicionado para preencher telas.

## Verificações realizadas

| Verificação | Resultado real |
| --- | --- |
| Download/preparo das dependências Go | Concluído; `go mod tidy` terminou com sucesso usando proxy local |
| Instalação das dependências npm | Concluída; `npm install --offline --ignore-scripts` instalou 115 pacotes e gerou lockfile |
| `go test ./internal/chat` | Passou, executado pelo subagente com HTTP e Vault simulados |
| `mygo generate` | Passou com configuração de ambiente temporária e gerou `src/mygo.ts` |
| `npm run lint` | **Falhou** em `IconButton`, detalhe abaixo |
| Geração após a última alteração em `main.go` | Ainda não repetida |
| `go vet ./...`, testes de todos os pacotes, `make check` | Ainda não executados |
| `vite build`, build desktop, build dos três sistemas | Ainda não executados |
| Abrir interface, conferir console, teclado, contraste e temas | Ainda não realizado |
| Login Google, histórico e envio reais | Ainda não realizado |

Os quatro testes existentes cobrem seleção do bearer token pela conta informada, remoção/reconexão com troca de sessão, preservação do ID de envio e rejeição de espaço inválido antes de chamar o transporte. A cobertura é pontual; não prova equivalência com o cliente Google nem cobre todas as situações de concorrência.

### Falha imediata de TypeScript

Última saída de `npm run lint`:

```text
src/App.tsx(36,94): error TS2322:
Type 'ReactNode' is not assignable to type 'WithSlotShorthandValue<...>'.
Type 'bigint' is not assignable ...
```

`IconButton` declara `children: ReactNode` e entrega esse valor ao slot `icon` do Fluent UI. A próxima correção prevista é restringir esse parâmetro a `ReactElement` ou ao tipo do slot. Não foi aplicada antes de o usuário pedir o handoff.

### Última alteração em main.go

O gerador de bindings executa o programa em modo `MYGO_GENERATE`. Inicialmente o código tentava abrir o diretório de dados antes de registrar o serviço, causando erro no sandbox.

O código foi reorganizado para registrar `Desktop` primeiro e abrir configuração/serviço dentro de `App.WhenReady`, antes de criar a janela. O modo de geração do MyGo termina antes desse callback. Essa alteração está escrita, mas falta repetir a geração para conferir seu resultado.

## Ambiente e como retomar neste sandbox

Diretório: `/home/ruivalim/Work/ruivalim/google-chat-client`.

Arquivos do projeto e `/tmp` são graváveis. `.git` está exposto como diretório somente leitura e não funciona como um repositório normal: Go falhou ao tentar obter status VCS. Não houve commit.

A rede direta de Go/Bun falhou por restrição de sockets/DNS. Downloads diretos autorizados com `curl -L` funcionaram. Para compilar localmente, dependências oficiais foram baixadas para `/tmp`, sem alterar caches do usuário.

Artefatos temporários disponíveis nesta máquina:

| Caminho | Uso |
| --- | --- |
| `/tmp/google-chat-client-gomod` | Cache de módulos Go |
| `/tmp/google-chat-client-gocache` | Cache de compilação Go |
| `/tmp/google-chat-client-go` | GOPATH temporário |
| `/tmp/google-chat-goproxy` | Proxy Go em arquivos, preenchido com arquivos oficiais de proxy.golang.org |
| `/tmp/google-chat-npm-cache` | Cache npm preparado para instalação offline |
| `/tmp/google-chat-deps` | Downloads e helper temporário de preparo do cache npm |
| `/tmp/google-chat-mygo-source/github.com/egoist/mygo@v0.5.5` | Fonte MyGo extraída para consultar APIs e exemplos |
| `/tmp/google-chat-client-config` | Diretório de configuração temporário usado durante uma geração |

`/tmp` pode não sobreviver a outra máquina/sessão. `node_modules/` existe no workspace e não é versionado. `src/mygo.ts` é gerado e também não é versionado. As fontes e lockfiles são os registros permanentes.

Comandos de retomada neste ambiente, **depois de corrigir o erro TypeScript**:

```bash
export GOFLAGS=-buildvcs=false
export GOMODCACHE=/tmp/google-chat-client-gomod
export GOCACHE=/tmp/google-chat-client-gocache
export GOPATH=/tmp/google-chat-client-go
export GOTOOLCHAIN=local
export GOPROXY=file:///tmp/google-chat-goproxy
export GOSUMDB=off

npm run generate
npm run lint
```

O `GOSUMDB=off` acima foi usado somente para a preparação offline deste ambiente restrito. Não adicioná-lo como configuração padrão ao projeto; em um ambiente normal, manter a verificação padrão do Go.

Se for preciso reinstalar a partir do cache temporário:

```bash
env NODE_PATH=/tmp/google-chat-deps npm install \
  --offline --ignore-scripts \
  --cache /tmp/google-chat-npm-cache \
  --logs-dir /tmp/google-chat-npm-logs \
  --no-audit --no-fund
```

`NODE_PATH` foi necessário para uma dependência `semver` ausente na instalação global de npm desta máquina. O helper/download está em `/tmp/google-chat-deps/semver`. Isso é uma particularidade local, não um requisito normal do aplicativo.

Em ambiente normal, os comandos pretendidos são `make setup`, `make dev`, `make check` e `make build-all`. O Makefile ainda precisa ser exercitado. Ele foi criado seguindo a skill makefile, mas **não existe workflow CI ainda**, apesar de a descrição do alvo `check` já mencionar CI; acrescentar o workflow ou corrigir a descrição.

O MyGo também precisa de Bun para ler a configuração TypeScript e executar as tarefas. No Linux, runtime de WebKitGTK/GTK e um Secret Service com libsecret são necessários para webview e credenciais. No Windows, conferir WebView2. Consultar os requisitos oficiais antes de escrever as instruções definitivas.

Para inspeção visual futura, há Playwright já disponível fora do projeto, por exemplo em `/home/ruivalim/.npm/_npx/e41f203b7505f1fb/node_modules/playwright`, e navegadores em `/home/ruivalim/.cache/ms-playwright`. Não foi acrescentada dependência Playwright ao projeto nem iniciado um teste visual. Não assumir que sockets ou GUI estarão disponíveis no mesmo sandbox.

## Pendências, em ordem sugerida

1. Corrigir o tipo do slot `icon`, regenerar a ponte e fazer TypeScript passar.
2. Formatar os arquivos Go/TS/CSS. O código inicial ainda não foi todo formatado.
3. Executar `go vet`, os testes do projeto e o build web. Resolver os problemas encontrados antes de tentar os três pacotes desktop.
4. Abrir e inspecionar a interface: console, primeira configuração, modal, teclado, tema claro/escuro, telas estreitas e 200% de zoom. Verificar o CSP no runtime MyGo.
5. Conferir em MyGo o isolamento do diretório de dados entre desenvolvimento e app instalado. Os secrets usam identificadores separados automaticamente, mas o uso de `SetName` e `PathUserData` ainda merece revisão para evitar metadados de uma variante apontando para tokens da outra.
6. Rever a troca muito rápida entre contas: respostas assíncronas de seleção devem respeitar a última intenção. O frontend já separa dados por componentes com chave da conta, mas ainda não foi exercitado esse cenário.
7. Revisar paginação, preservação de rolagem, mensagens removidas/atualizadas e resposta parcial em reenvio idempotente. O histórico usa merge e polling, que ainda precisam ser observados em uso.
8. Preparar e documentar o projeto Google Cloud na conta corporativa de teste, importar o JSON no app e validar login, nomes das DMs, leitura, envio, resposta em thread e remoção/reconexão.
9. Para contas de organizações diferentes, avaliar audiência OAuth **External** com usuários de teste autorizados. Um projeto com audiência Internal limita os usuários à sua organização. Não presumir que o usuário administrador já criou o projeto ou forneceu credenciais; isso ainda não aconteceu.
10. Empacotar Linux x64, Windows x64 e macOS universal, validando em cada sistema. A assinatura e a notarização para distribuição ficam para uma etapa própria. Não prometer instaladores que ainda não foram gerados.
11. Atualizar README e contextualizar a pesquisa antiga com a escolha MyGo/Fluent UI. Documentar setup, escopos, funcionamento do chaveiro, limitações e caminhos de retomada.
12. Acrescentar CI com os mesmos checks do Makefile; confirmar versões dos actions e exigências de runtime nos runners.

## Funcionalidades futuras e limites atuais

- A atualização atual é polling apenas da conversa aberta. **Não há notificações de todas as contas em segundo plano.**
- Próxima evolução planejada: receber eventos via Workspace Events API e Cloud Pub/Sub, renovar assinaturas e recuperar alterações após desconexão; decidir entre consumo local e um serviço de apoio.
- Não há UI de criar uma DM/espaço, reações, edição/exclusão de mensagem ou busca no servidor. O filtro atual só filtra a lista de conversas carregada.
- Não há sincronização de leitura/não lidas. Não pedir o escopo correspondente sem implementar a função.
- Anexos aparecem como nomes, mas não há upload/download integrado. Cards e outros conteúdos especiais não são renderizados completamente; há abertura da conversa no Google para acesso ao conteúdo original.
- Mensagens são exibidas como texto e links, sem renderização completa da formatação Google/markdown.
- Não há banco/cache offline de mensagens. Somente metadados e preferências são persistentes; rascunhos ficam em memória.
- Disponibilidade de colegas e indicador de digitação não devem ser inventados: a documentação consultada de disponibilidade trata do próprio usuário e não foi localizado método público para digitação no catálogo v1.
- Calendar: agenda e eventos em fase posterior.
- Meet: começar pela abertura de links de reunião no navegador; integrar mídia dentro do app exige investigação adicional.
- Gmail: fase posterior com autorização e requisitos próprios.

## Skills e colaboração

- Core: `/home/ruivalim/.codex/skills/antislop/SKILL.md`.
- UI: `/home/ruivalim/.codex/skills/antislop-ui/SKILL.md`.
- Acessibilidade: `/home/ruivalim/.codex/skills/antislop-human/SKILL.md`.
- Makefile: `/home/ruivalim/.agents/skills/makefile/SKILL.md` e seu template.

A escolha de usar antislop durante a construção já foi respondida pelo usuário; não perguntar de novo. A direção visual está em `docs/design.md`: cliente de trabalho, Fluent UI, contas sempre acessíveis, verde como acento, neutros, fontes do sistema e ENERGY 1 / RHYTHM 1 / MOTION 1. Ainda não há relatório de aprovação visual; não declarar PASS sem observar e validar a interface.

O único subagente, `review_accounts`, usou `gpt-6-luna` e escreveu os testes em `internal/chat/service_test.go`. Terminou sem encontrar defeito de produção na revisão delimitada. Não há trabalho de subagente pendente.

## Fontes principais para continuar

- [MyGo: início](https://mygo.egoist.dev/docs/getting-started)
- [MyGo: credenciais e APIs desktop](https://mygo.egoist.dev/docs/native)
- [MyGo: bindings](https://mygo.egoist.dev/docs/bindings)
- [MyGo: configuração](https://mygo.egoist.dev/docs/configuration)
- [MyGo: distribuição](https://mygo.egoist.dev/docs/distribution)
- [Google: OAuth para desktop](https://developers.google.com/identity/protocols/oauth2/native-app)
- [Google Chat: autenticação como usuário](https://developers.google.com/workspace/chat/authenticate-authorize-chat-user)
- [Google Chat: métodos e escopos](https://developers.google.com/workspace/chat/authenticate-authorize)
- [Google Chat: catálogo de descoberta v1](https://chat.googleapis.com/$discovery/rest?version=v1)
- [Google Chat: eventos](https://developers.google.com/workspace/events/guides/events-chat)
- [Workspace: eventos e entrega](https://developers.google.com/workspace/events)
- [Workspace: controle de acesso OAuth](https://knowledge.workspace.google.com/admin/apps/control-which-third-party-and-internal-apps-access-google-workspace-data)
- [Fluent UI](https://github.com/microsoft/fluentui)

## Sugestão de mensagem para retomar

> Leia HANDOFF.md e continue de onde a sessão parou. Mantenha MyGo + Go + React + Fluent UI e múltiplas contas desde a base. Corrija o erro TypeScript registrado, valide a interface e prepare o app para conectar minha conta corporativa de teste. Pode usar poucos subagentes mais baratos. A aplicação de antislop durante a construção já foi aprovada.
