# Google Chat Client

App desktop para usar o Google Chat com várias contas Google Workspace na mesma janela, inclusive contas de empresas diferentes. Cada conta tem a própria sessão, as próprias conversas e os próprios rascunhos. Roda em Linux, Windows e macOS.

Chat vem primeiro. Calendar, Meet e Gmail ficam para depois.

> **Estado:** compila, abre e passa nos testes, mas ainda não foi usado com uma conta Google de verdade. Os testes simulam o Google e a ponte com o Go. O primeiro login real é o próximo passo, e a lista do que conferir nele está [no fim](#o-que-falta).

## Como é feito

| Parte | O quê |
| --- | --- |
| Janela e empacotamento | [MyGo](https://mygo.egoist.dev) 0.5.5, com a webview do sistema (WebKitGTK, WebView2, WKWebView) |
| Backend | Go 1.27: OAuth, chaveiro do sistema e chamadas à API do Chat |
| Interface | React 19, TypeScript 7, [Fluent UI](https://react.fluentui.dev) 9 |
| Ponte | O MyGo gera `src/mygo.ts` a partir dos métodos de `desktop.go` |

Token e client secret nunca chegam à interface: ficam no Go e no chaveiro. O `accounts.json` guarda só os projetos importados (client ID e nome) e, de cada conta, nome, email, domínio e o projeto por onde ela entrou.

```
main.go, desktop.go     janela, menus e os métodos que a interface chama
internal/chat/          OAuth, sessões por conta e API do Google Chat
src/App.tsx             interface inteira
e2e/                    testes de interface com a ponte simulada
docs/                   direção visual e o que a API permite
```

## Rodar

Precisa de Go 1.27+, [Bun](https://bun.sh) (o MyGo usa para ler a config e rodar as tarefas) e Node 24+. No Linux, também GTK 3, WebKitGTK 4.1 e um Secret Service para o chaveiro (GNOME Keyring, KWallet ou KeePassXC). No Debian e no Ubuntu: `sudo apt install libwebkit2gtk-4.1-0 libsecret-1-0`. No Windows 10 falta o [WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/); o 11 já vem com ele.

```sh
make setup      # dependências e o Chromium dos testes
make dev        # app com recarga automática
make preview    # só a interface no navegador, sem login
make check      # lint, testes e build: o mesmo que o CI roda
make build      # pacote da plataforma atual em build/
make build-all  # Linux x64, Windows x64 e macOS universal
```

O `make dev` grava em `Google Chat Client Dev` e o app instalado em `Google Chat Client`, no diretório de configuração do sistema (`~/.config` no Linux, `~/Library/Application Support` no macOS, `%AppData%` no Windows). Os segredos de cada um ficam em serviços separados do chaveiro, então dá para mexer no dev sem tocar nas contas do app de uso.

O `build-all` feito no Linux gera tudo, menos o instalador do Windows (precisa do `nsis`) e o `.dmg` (só sai em macOS). O CI gera os dois: cada push na `main` deixa os pacotes dos três sistemas como artefato da run.

## Configurar o Google Cloud

O app não vem com credencial embutida: você cria o projeto e importa o JSON. O mesmo passo a passo, com links para cada página do console e um botão de copiar os escopos, fica no app em **Como conectar**.

1. **Crie um projeto só para este app.** Não reaproveite projeto de outra coisa: o nome que aparece no login (Branding) e as regras de acesso valem para tudo o que usa o projeto. Para ter público **Interno**, crie logado com a conta da empresa, dentro da organização dela; projeto pessoal só oferece Externo.
2. **Ative a Google Chat API** e, na aba Configuração dela, preencha nome, URL de um ícone e descrição. Sem a API ativada nada funciona (`SERVICE_DISABLED`). Sem a configuração, a leitura funciona mas o envio falha com "Google Chat app not found".
3. **Google Auth Platform:**
   - **Branding:** o nome que aparece no login e seu email.
   - **Público:** **Interno** aceita só contas da organização dona do projeto, e quem é de fora toma `org_internal`. **Externo** aceita contas de qualquer empresa.
   - **Acesso a dados:** os escopos da tabela abaixo.
4. **Com público Externo, escolha teste ou produção:**
   - **Em teste**, só entra quem estiver em Público > Usuários de teste (até 100). Conta fora da lista vê "has not completed the Google verification process (...) can only be accessed by developer-approved testers". A sessão vence a cada 7 dias.
   - **Em produção** (Público > Publicar app), a sessão não vence toda semana. Sem verificação, o login mostra a tela de app não verificado, com teto de 100 usuários na vida do projeto. A documentação diz que escopo restrito precisa de verificação em produção; se o Google bloquear o login, volte para teste.
5. **Clientes:** crie um cliente do tipo **Aplicativo para computador**, baixe o JSON, importe no app (Configurações > **Importar arquivo JSON**) e use **Adicionar conta Google**. O login abre no navegador do sistema.
   Para fazer o login em outro navegador ou num container (do Firefox ou do Zen, por exemplo), use **Copiar link** enquanto o login está pendente e cole lá. Copie por esse botão: a URL de uma etapa já começada no navegador não funciona em outro container, e o Google responde com um erro 400.

### Mais de um projeto

Dá para importar vários JSON, um por projeto, e cada conta entra pelo projeto que você escolher em Configurações > Projetos Google Cloud. Serve para dois casos:

- **Fugir dos 7 dias onde dá.** Na empresa em que você é admin, um projeto com público **Interno** não tem sessão vencendo toda semana. As outras empresas continuam num projeto **Externo**.
- **Empresa que exige projeto próprio.** Se o admin de uma empresa só libera app de projeto dela, importe o JSON desse projeto e conecte a conta por ele.

Cada conta renova a sessão com as credenciais do projeto por onde entrou. Um projeto só sai quando nenhuma conta usa ele; para trocar uma conta de projeto, basta conectá-la de novo pelo outro.

| Escopo | Por quê |
| --- | --- |
| `openid`, `email`, `profile` | Saber qual conta entrou |
| `chat.spaces.readonly` | Listar conversas e espaços |
| `chat.memberships.readonly` | Dar nome às DMs e grupos, que a API entrega sem nome |
| `chat.messages` | Ler e enviar mensagens |

### O que esperar do Google

- **Em modo de teste, a sessão vence em 7 dias.** Com público Externo e status *Testing*, o Google emite refresh token que expira em uma semana. Quando expirar, o app avisa e basta adicionar a conta de novo. A documentação só cita essa regra para o modo de teste.
- **`chat.messages` é escopo restrito.** Publicado sem verificação, o app mostra a tela de "app não verificado" e aceita no máximo 100 usuários pela vida inteira do projeto. A política diz que escopo restrito não pode ser usado em produção sem revisão; não achei na documentação se o Google só avisa ou bloqueia o login nesse caso, então produção é algo a testar. A verificação de escopo restrito é um processo à parte.
- **Cada empresa pode bloquear o app.** Se o login de uma conta corporativa der `admin_policy_enforced`, o admin daquele Workspace precisa marcar o client ID como confiável em Admin console > Segurança > Acesso e controle de dados > Controles de API.

Fontes: [publicação e teto de usuários](https://support.google.com/cloud/answer/15549945), [política de escopos em produção](https://developers.google.com/identity/protocols/oauth2/production-readiness/policy-compliance), [expiração de refresh token](https://developers.google.com/identity/protocols/oauth2#expiration), [público e usuários de teste](https://support.google.com/cloud/answer/15549945), [classificação dos escopos do Chat](https://developers.google.com/workspace/chat/authenticate-authorize), [controle de apps no Workspace](https://knowledge.workspace.google.com/admin/apps/control-which-apps-access-google-workspace-data).

## O que funciona

- Várias contas, troca pela faixa da esquerda, cada uma identificada pela cor e pelas iniciais da organização.
- Vários projetos Google Cloud, cada conta no seu.
- Lista de conversas paginada, com filtro local e abas de conversas e espaços.
- Histórico paginado, threads em painel lateral, envio e resposta em thread.
- Reenvio seguro: se o envio falhar, o texto fica e a nova tentativa usa o mesmo `requestId`, então o Google não duplica a mensagem.
- Rascunho separado por conta, conversa e thread (só em memória).
- Tema claro, escuro ou do sistema; teclado e leitor de tela tratados.
- A conversa aberta se atualiza a cada 15 segundos.

## Testes

`make test` roda as duas metades:

- **Go** (`internal/chat`, com `-race`): o fluxo OAuth inteiro contra um Google simulado (callback com `state` forjado, consentimento negado, bloqueio do admin, escopo faltando, identidade inválida), renovação de token concorrente, sessão vencida, erros HTTP, JSON truncado e config corrompida.
- **Interface** (`e2e/`, Playwright): o build de produção com a ponte simulada. Troca rápida de conta, reenvio idempotente, rascunho por conversa, Enter e Shift+Enter com IME, thread, links e erro com nova tentativa.

## O que falta

No primeiro login real (Workspace, projeto Externo em teste) já funcionaram: login, lista de conversas, histórico paginado, nome dos remetentes e reconhecimento das próprias mensagens. Falta conferir:

- Nome das DMs e dos grupos na lista de conversas.
- Envio e resposta em thread, depois de configurar o app de Chat.
- Remover e reconectar conta.
- Login de uma segunda organização no mesmo projeto Externo, e de uma conta por um projeto Interno.
- Instaladores rodando de fato no Windows e no macOS (até agora só foram compilados).

Depois disso, em ordem:

1. Eventos em tempo real (Workspace Events API + Pub/Sub) e notificações de todas as contas, não só da conversa aberta.
2. Não lidas, com `chat.users.readstate`.
3. Anexos (hoje só aparece o nome), formatação das mensagens e cards.
4. Editar, apagar, reagir, buscar e começar conversa nova.
5. Assinatura dos pacotes para distribuir (Developer ID no macOS, certificado no Windows).
6. Calendar, Meet (começando por abrir o link) e Gmail.

O que a API permite e o que não existe nela (indicador de digitação, presença dos colegas) está em [docs/google-chat-api.md](docs/google-chat-api.md). As decisões visuais estão em [docs/design.md](docs/design.md).
