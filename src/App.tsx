import {
  Avatar,
  Button,
  Dialog,
  DialogActions,
  DialogBody,
  DialogContent,
  DialogSurface,
  DialogTitle,
  FluentProvider,
  Input,
  MessageBar,
  MessageBarBody,
  Select,
  Spinner,
  Tooltip,
  webDarkTheme,
  webLightTheme,
} from "@fluentui/react-components";
import {
  Add24Regular,
  ArrowClockwise24Regular,
  ArrowLeft24Regular,
  ArrowUpload24Regular,
  Chat24Regular,
  ChatMultiple24Regular,
  Dismiss24Regular,
  Open24Regular,
  People24Regular,
  Search24Regular,
  Send24Regular,
  Settings24Regular,
  WeatherMoon24Regular,
  WeatherSunny24Regular,
} from "@fluentui/react-icons";
import { isMyGo } from "mygo-runtime";
import { type ReactElement, type ReactNode, useEffect, useLayoutEffect, useRef, useState } from "react";
import { type Account, Desktop, type Message, type Space, type State } from "./mygo";

const emptyState: State = {
  configured: false,
  clientId: "",
  accounts: [],
  activeAccountId: "",
};
const native = isMyGo();
const drafts = new Map<string, { text: string; requestId: string }>();
const selectedSpaces = new Map<string, Space>();
type Appearance = "system" | "light" | "dark";

// As mensagens vêm do Go em minúscula, como manda a convenção de lá.
function describe(error: unknown) {
  const text = error instanceof Error ? error.message : String(error);
  return text.charAt(0).toLocaleUpperCase("pt-BR") + text.slice(1);
}

// Duas contas da mesma pessoa só se distinguem pela organização, então a faixa
// mostra as iniciais e a cor do domínio. Conta pessoal, sem domínio, usa o nome.
function AccountAvatar({ account, size }: { account: Account; size?: 36 }) {
  const org = account.domain.split(".")[0] ?? "";
  const parts = org.split(/[-_]/).filter(Boolean);
  const initials = parts.length > 1 ? parts[0][0] + parts[1][0] : org.slice(0, 2);
  return (
    <Avatar
      aria-hidden
      name={account.name || account.email}
      initials={initials ? initials.toLocaleUpperCase("pt-BR") : undefined}
      color="colorful"
      idForColor={account.domain || account.email}
      size={size}
    />
  );
}

function senderName(m: Message, account: Account) {
  if (m.sender.name === `users/${account.id}`) return account.name || account.email;
  if (m.sender.displayName) return m.sender.displayName;
  if (m.sender.email) return m.sender.email;
  if (m.sender.type === "BOT") return "Aplicativo";
  return m.sender.isAnonymous ? "Perfil indisponível" : "Pessoa sem nome visível";
}

function openLink(url: string) {
  if (native) return Desktop.openLink(url);
  window.open(url, "_blank", "noopener,noreferrer");
  return Promise.resolve();
}

function IconButton({
  label,
  children,
  ...props
}: {
  label: string;
  children: ReactElement;
  onClick: () => void;
  disabled?: boolean;
  appearance?: "subtle" | "secondary";
  className?: string;
}) {
  return (
    <Tooltip content={label} relationship="label">
      <Button {...props} aria-label={label} icon={children} />
    </Tooltip>
  );
}

function Notice({ children }: { children: ReactNode }) {
  return (
    <MessageBar intent="error" layout="multiline">
      <MessageBarBody>{children}</MessageBarBody>
    </MessageBar>
  );
}

export function App() {
  const [state, setState] = useState<State>(emptyState);
  const selection = useRef(0);
  const selectQueue = useRef<Promise<unknown>>(Promise.resolve());
  const [starting, setStarting] = useState(native);
  const [error, setError] = useState("");
  const [settings, setSettings] = useState(false);
  const [connecting, setConnecting] = useState(false);
  const [importing, setImporting] = useState(false);
  const [removing, setRemoving] = useState("");
  const [appearance, setAppearance] = useState<Appearance>(() => {
    const saved = localStorage.getItem("appearance");
    return saved === "light" || saved === "dark" ? saved : "system";
  });
  const [systemDark, setSystemDark] = useState(matchMedia("(prefers-color-scheme: dark)").matches);
  const dark = appearance === "system" ? systemDark : appearance === "dark";
  const account = state.accounts.find((a) => a.id === state.activeAccountId) ?? state.accounts[0];
  const theme = dark
    ? {
        ...webDarkTheme,
        colorBrandBackground: "#006b4c",
        colorBrandBackgroundHover: "#00845e",
        colorBrandForeground1: "#77dfb5",
        colorBrandForeground2: "#a2edd0",
        colorBrandStroke1: "#77dfb5",
      }
    : {
        ...webLightTheme,
        colorBrandBackground: "#006b4c",
        colorBrandBackgroundHover: "#00513a",
        colorBrandBackgroundPressed: "#003b2a",
        colorBrandForeground1: "#006b4c",
        colorBrandForeground2: "#00513a",
        colorBrandStroke1: "#006b4c",
      };

  useEffect(() => {
    if (!native) return;
    Desktop.state()
      .then(setState)
      .catch((err) => setError(describe(err)))
      .finally(() => setStarting(false));
  }, []);
  useEffect(() => {
    const query = matchMedia("(prefers-color-scheme: dark)");
    const change = () => setSystemDark(query.matches);
    query.addEventListener("change", change);
    return () => query.removeEventListener("change", change);
  }, []);
  useEffect(() => {
    document.documentElement.dataset.theme = dark ? "dark" : "light";
    localStorage.setItem("appearance", appearance);
  }, [appearance, dark]);

  async function connect() {
    if (!state.configured) {
      setSettings(true);
      return;
    }
    setError("");
    setConnecting(true);
    try {
      const next = await Desktop.connectAccount();
      selection.current++;
      setState(next);
      setSettings(false);
    } catch (err) {
      setError(describe(err));
    } finally {
      setConnecting(false);
    }
  }

  async function importCredentials() {
    setError("");
    setImporting(true);
    try {
      setState(await Desktop.importCredentials());
    } catch (err) {
      setError(describe(err));
    } finally {
      setImporting(false);
    }
  }

  // Cliques seguidos na faixa de contas: a tela troca na hora, as chamadas ao Go
  // saem em fila para a conta gravada ser a do último clique, e só a resposta
  // desse último clique atualiza o estado.
  async function select(id: string) {
    const turn = ++selection.current;
    setError("");
    setState((current) => ({ ...current, activeAccountId: id }));
    const request = selectQueue.current.then(() => Desktop.selectAccount(id));
    selectQueue.current = request.catch(() => undefined);
    try {
      const next = await request;
      if (turn === selection.current) setState(next);
    } catch (err) {
      if (turn !== selection.current) return;
      setError(describe(err));
      Desktop.state()
        .then((next) => turn === selection.current && setState(next))
        .catch(() => undefined);
    }
  }

  async function remove(id: string) {
    setRemoving(id);
    setError("");
    try {
      const next = await Desktop.removeAccount(id);
      selection.current++;
      setState(next);
      selectedSpaces.delete(id);
      for (const key of drafts.keys()) if (key.startsWith(`${id}|`)) drafts.delete(key);
    } catch (err) {
      setError(describe(err));
    } finally {
      setRemoving("");
    }
  }

  return (
    <FluentProvider theme={theme} className="app">
      <a href="#main-content" className="skip-link">
        Ir para a conversa
      </a>
      <nav className="account-rail" aria-label="Contas Google">
        <div className="rail-heading" title="Google Chat Client">
          <ChatMultiple24Regular />
        </div>
        <div className="account-buttons">
          {state.accounts.map((a) => (
            <Tooltip key={a.id} content={`${a.name || a.email} · ${a.email}`} relationship="label">
              <button
                type="button"
                className={`account-button ${account?.id === a.id ? "selected" : ""}`}
                aria-pressed={account?.id === a.id}
                aria-label={`Usar ${a.email}`}
                onClick={() => void select(a.id)}
              >
                <AccountAvatar account={a} size={36} />
              </button>
            </Tooltip>
          ))}
        </div>
        <IconButton
          label="Adicionar conta Google"
          appearance="subtle"
          disabled={!native || connecting}
          onClick={() => void connect()}
        >
          <Add24Regular />
        </IconButton>
        <div className="rail-bottom">
          <IconButton
            label={dark ? "Usar tema claro" : "Usar tema escuro"}
            appearance="subtle"
            onClick={() => setAppearance(dark ? "light" : "dark")}
          >
            {dark ? <WeatherSunny24Regular /> : <WeatherMoon24Regular />}
          </IconButton>
          <IconButton label="Configurações e contas" appearance="subtle" onClick={() => setSettings(true)}>
            <Settings24Regular />
          </IconButton>
        </div>
      </nav>
      <main id="main-content" className="main" tabIndex={-1}>
        {error && !settings && (
          <div className="global-notice">
            <Notice>{error}</Notice>
          </div>
        )}
        {connecting && !settings && (
          <div className="connection-status" role="status">
            <Spinner size="tiny" /> Conclua o login no navegador{" "}
            <Button size="small" onClick={() => void Desktop.cancelLogin()}>
              Cancelar
            </Button>
          </div>
        )}
        {starting ? (
          <div className="center-state">
            <Spinner label="Carregando suas contas" />
          </div>
        ) : account ? (
          <Workspace key={account.id} account={account} onSettings={() => setSettings(true)} />
        ) : (
          <div className="welcome">
            <div className="welcome-heading">
              <ChatMultiple24Regular />
              <span>Google Chat Client</span>
            </div>
            <div className="welcome-content">
              <p className="eyebrow">Seu espaço de trabalho</p>
              <h1>
                Suas conversas.
                <br />
                Cada conta no seu lugar.
              </h1>
              <p className="welcome-description">
                Conecte suas contas Google Workspace e alterne entre elas na mesma janela.
              </p>
              {!native && (
                <div className="preview-note">
                  Abra o aplicativo desktop para conectar suas contas. Esta janela mostra apenas a interface.
                </div>
              )}
              <div className="welcome-action">
                {state.configured ? (
                  <Button
                    appearance="primary"
                    size="large"
                    icon={<Add24Regular />}
                    disabled={!native || connecting}
                    onClick={() => void connect()}
                  >
                    Conectar conta Google
                  </Button>
                ) : (
                  <Button
                    appearance="primary"
                    size="large"
                    icon={<ArrowUpload24Regular />}
                    disabled={!native || importing}
                    onClick={() => void importCredentials()}
                  >
                    Importar credenciais Google
                  </Button>
                )}
                <Button appearance="subtle" onClick={() => setSettings(true)}>
                  Como conectar
                </Button>
              </div>
              <p className="welcome-detail">
                {state.configured
                  ? "Projeto configurado. O login abre no seu navegador."
                  : "Primeiro, importe o arquivo OAuth do seu projeto Google Cloud."}
              </p>
            </div>
            <div className="welcome-footer">
              <span>Chat para o trabalho</span>
              <span>Windows · Linux · macOS</span>
            </div>
          </div>
        )}
      </main>
      <Dialog open={settings} onOpenChange={(_, data) => setSettings(data.open)}>
        <DialogSurface className="settings-dialog">
          <DialogBody>
            <DialogTitle
              action={
                <IconButton label="Fechar configurações" appearance="subtle" onClick={() => setSettings(false)}>
                  <Dismiss24Regular />
                </IconButton>
              }
            >
              Configurações
            </DialogTitle>
            <DialogContent>
              {error && <Notice>{error}</Notice>}
              <section className="settings-section">
                <h2>Contas Google</h2>
                <p>As contas mantêm sessões separadas. Remover uma conta encerra a sessão neste aplicativo.</p>
                {state.accounts.map((a) => (
                  <div className="settings-account" key={a.id}>
                    <AccountAvatar account={a} />
                    <div>
                      <strong>{a.name || a.email}</strong>
                      <span>{a.email}</span>
                    </div>
                    <Button disabled={removing !== "" || connecting} onClick={() => void remove(a.id)}>
                      {removing === a.id ? "Removendo…" : "Remover"}
                    </Button>
                  </div>
                ))}
                {connecting ? (
                  <div className="login-pending" role="status">
                    <Spinner size="tiny" label="Conclua o login no navegador" />
                    <Button onClick={() => void Desktop.cancelLogin()}>Cancelar</Button>
                  </div>
                ) : (
                  <Button
                    icon={<Add24Regular />}
                    disabled={!native || !state.configured || importing}
                    onClick={() => void connect()}
                  >
                    Adicionar conta Google
                  </Button>
                )}
              </section>
              <section className="settings-section">
                <h2>Projeto Google Cloud</h2>
                <p>
                  Habilite a Google Chat API e crie um cliente OAuth do tipo <strong>Aplicativo para computador</strong>
                  . Baixe o JSON e importe aqui.
                </p>
                <div className="settings-actions">
                  <Button
                    icon={<ArrowUpload24Regular />}
                    disabled={!native || importing || connecting}
                    onClick={() => void importCredentials()}
                  >
                    {importing ? "Importando…" : "Importar arquivo JSON"}
                  </Button>
                  <Button
                    appearance="subtle"
                    icon={<Open24Regular />}
                    onClick={() => void openLink("https://console.cloud.google.com/auth/clients")}
                  >
                    Abrir Google Cloud
                  </Button>
                </div>
                <p className="configuration-state">
                  {state.configured ? "Projeto configurado" : "Nenhum projeto configurado"}
                </p>
                {state.clientId && <code className="client-id">{state.clientId}</code>}
                <p>
                  Para contas de mais de uma organização, o público do projeto precisa ser <strong>Externo</strong>, com
                  cada conta como usuária de teste. Nesse modo o Google encerra a sessão a cada 7 dias e a conta precisa
                  ser adicionada de novo.
                </p>
                <p>
                  O login pode depender da liberação do aplicativo pelo administrador do Workspace. As credenciais ficam
                  no chaveiro do sistema.
                </p>
              </section>
              <section className="settings-section appearance-setting">
                <label htmlFor="appearance">Aparência</label>
                <Select
                  id="appearance"
                  value={appearance}
                  onChange={(_, data) => setAppearance(data.value as Appearance)}
                >
                  <option value="system">Seguir o sistema</option>
                  <option value="light">Claro</option>
                  <option value="dark">Escuro</option>
                </Select>
              </section>
            </DialogContent>
            <DialogActions>
              <Button appearance="primary" onClick={() => setSettings(false)}>
                Concluir
              </Button>
            </DialogActions>
          </DialogBody>
        </DialogSurface>
      </Dialog>
    </FluentProvider>
  );
}

function Workspace({ account, onSettings }: { account: Account; onSettings: () => void }) {
  const [spaces, setSpaces] = useState<Space[]>([]);
  const [cursor, setCursor] = useState("");
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState("");
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState("all");
  const [selected, setSelected] = useState<Space | null>(selectedSpaces.get(account.id) ?? null);
  const [thread, setThread] = useState("");
  const [refresh, setRefresh] = useState(0);
  const alive = useRef(true);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);
  // biome-ignore lint/correctness/useExhaustiveDependencies: refresh existe só para disparar a busca de novo
  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError("");
    Desktop.spaces(account.id, "")
      .then((page) => {
        if (cancelled) return;
        setSpaces(page.spaces ?? []);
        setCursor(page.nextPageToken);
      })
      .catch((err) => {
        if (!cancelled) setError(describe(err));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [account.id, refresh]);

  function select(space: Space) {
    selectedSpaces.set(account.id, space);
    setSelected(space);
    setThread("");
  }
  async function more() {
    setLoadingMore(true);
    setError("");
    try {
      const page = await Desktop.spaces(account.id, cursor);
      if (alive.current) {
        setSpaces((old) => [...new Map([...old, ...(page.spaces ?? [])].map((s) => [s.name, s])).values()]);
        setCursor(page.nextPageToken);
      }
    } catch (err) {
      if (alive.current) setError(describe(err));
    } finally {
      if (alive.current) setLoadingMore(false);
    }
  }
  const filtered = spaces.filter((s) => {
    const type = filter === "all" || (filter === "spaces" ? s.spaceType === "SPACE" : s.spaceType !== "SPACE");
    return type && spaceName(s).toLocaleLowerCase().includes(query.toLocaleLowerCase());
  });
  return (
    <div className="workspace" data-has-space={!!selected}>
      <aside className="conversations" aria-label="Conversas da conta selecionada">
        <header className="workspace-heading">
          <div>
            <h1>Chat</h1>
            <button type="button" className="account-label" onClick={onSettings} title={account.email}>
              {account.domain || account.email}
            </button>
          </div>
          <IconButton
            label="Atualizar lista de conversas"
            appearance="subtle"
            disabled={loading}
            onClick={() => setRefresh((n) => n + 1)}
          >
            <ArrowClockwise24Regular />
          </IconButton>
        </header>
        <div className="conversation-filter">
          <Input
            contentBefore={<Search24Regular />}
            placeholder="Filtrar conversas"
            aria-label="Filtrar conversas carregadas"
            value={query}
            onChange={(_, data) => setQuery(data.value)}
          />
          <fieldset className="filter-tabs" aria-label="Tipo de conversa">
            {[
              ["all", "Tudo"],
              ["chats", "Conversas"],
              ["spaces", "Espaços"],
            ].map(([value, label]) => (
              <button type="button" key={value} aria-pressed={filter === value} onClick={() => setFilter(value)}>
                {label}
              </button>
            ))}
          </fieldset>
        </div>
        {error && (
          <div className="list-notice">
            <Notice>{error}</Notice>
            <Button onClick={() => setRefresh((n) => n + 1)}>Tentar novamente</Button>
          </div>
        )}
        <div className="conversation-list" aria-busy={loading}>
          {loading && spaces.length === 0 ? (
            <Spinner size="small" label="Buscando conversas" />
          ) : filtered.length ? (
            filtered.map((s) => (
              <button
                type="button"
                className={`conversation-row ${selected?.name === s.name ? "selected" : ""}`}
                key={s.name}
                onClick={() => select(s)}
                aria-current={selected?.name === s.name ? "page" : undefined}
              >
                <span className="space-avatar">
                  {s.spaceType === "SPACE" ? (
                    <People24Regular />
                  ) : (
                    <Avatar aria-hidden name={spaceName(s)} color="neutral" size={36} />
                  )}
                </span>
                <span className="conversation-label">
                  <strong>{spaceName(s)}</strong>
                  <span>
                    {s.spaceType === "SPACE"
                      ? "Espaço"
                      : s.spaceType === "GROUP_CHAT"
                        ? "Conversa em grupo"
                        : "Conversa direta"}
                  </span>
                </span>
              </button>
            ))
          ) : (
            !loading &&
            !error && (
              <div className="list-empty">
                {query || filter !== "all"
                  ? "Nenhuma conversa corresponde ao filtro."
                  : "Nenhuma conversa encontrada nesta conta. Inicie uma no Google Chat e atualize a lista."}
              </div>
            )
          )}
          {cursor && (
            <Button className="more-conversations" disabled={loadingMore} onClick={() => void more()}>
              {loadingMore ? "Carregando…" : "Mais conversas"}
            </Button>
          )}
        </div>
        <footer className="workspace-footer">
          <span title={account.email}>{account.email}</span>
          <span>Conta ativa</span>
        </footer>
      </aside>
      <section className="conversation" aria-label="Histórico da conversa">
        {selected ? (
          <>
            <header className="conversation-header">
              <IconButton
                label="Voltar às conversas"
                appearance="subtle"
                className="back-button"
                onClick={() => {
                  setSelected(null);
                  setThread("");
                }}
              >
                <ArrowLeft24Regular />
              </IconButton>
              <div className="conversation-title">
                <h2>{spaceName(selected)}</h2>
                <span>{account.email}</span>
              </div>
              {selected.spaceUri && (
                <IconButton
                  label="Abrir esta conversa no Google Chat"
                  appearance="subtle"
                  onClick={() => void openLink(selected.spaceUri)}
                >
                  <Open24Regular />
                </IconButton>
              )}
            </header>
            <div className="conversation-panels">
              <Timeline
                key={`${account.id}|${selected.name}`}
                account={account}
                space={selected}
                onThread={setThread}
              />
              {thread && (
                <aside className="thread-panel" aria-label="Thread">
                  <header>
                    <h3>Thread</h3>
                    <IconButton label="Fechar thread" appearance="subtle" onClick={() => setThread("")}>
                      <Dismiss24Regular />
                    </IconButton>
                  </header>
                  <Timeline key={`${account.id}|${thread}`} account={account} space={selected} thread={thread} />
                </aside>
              )}
            </div>
          </>
        ) : (
          <div className="center-state choose-conversation">
            <Chat24Regular />
            <h2>Uma conversa de cada vez</h2>
            <p>Escolha uma conversa à esquerda para ler e responder.</p>
            <span className="current-account">Você está usando {account.email}</span>
          </div>
        )}
      </section>
    </div>
  );
}

const linkPattern = /(https?:\/\/[^\s<>]*[^\s<>.,;:!?)\]'"])/g;

function spaceName(space: Space) {
  return space.displayName || `${space.spaceType === "SPACE" ? "Espaço" : "Conversa"} ${space.name.split("/").at(-1)}`;
}

function mergeMessages(previous: Message[], incoming: Message[], fresh: boolean) {
  if (fresh && incoming.length === 0) return [];
  const earliest = incoming[0]?.createTime;
  const retained = fresh ? previous.filter((m) => m.createTime < earliest) : previous;
  return [...new Map([...retained, ...incoming].map((m) => [m.name, m])).values()].sort((a, b) =>
    a.createTime.localeCompare(b.createTime),
  );
}

function Timeline({
  account,
  space,
  thread = "",
  onThread,
}: {
  account: Account;
  space: Space;
  thread?: string;
  onThread?: (name: string) => void;
}) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [cursor, setCursor] = useState("");
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState("");
  const [refresh, setRefresh] = useState(0);
  const scroller = useRef<HTMLElement>(null);
  const follow = useRef(true);
  const older = useRef(false);
  const alive = useRef(true);
  const position = useRef<{ top: number; height: number } | null>(null);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);
  // biome-ignore lint/correctness/useExhaustiveDependencies: refresh existe só para disparar a busca de novo
  useEffect(() => {
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout>;
    let failures = 0;
    async function update() {
      try {
        const page = await Desktop.messages(account.id, space.name, "", thread);
        if (cancelled) return;
        setMessages((previous) => mergeMessages(previous, page.messages ?? [], true));
        if (!older.current) setCursor(page.nextPageToken);
        setError("");
        failures = 0;
      } catch (err) {
        if (!cancelled) {
          setError(describe(err));
          failures += 1;
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
          timer = setTimeout(() => void update(), Math.min(15000 * 2 ** failures, 120000));
        }
      }
    }
    void update();
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [account.id, space.name, thread, refresh]);
  // biome-ignore lint/correctness/useExhaustiveDependencies: a rolagem precisa ser ajustada a cada troca de mensagens
  useLayoutEffect(() => {
    const node = scroller.current;
    if (!node) return;
    if (position.current) {
      node.scrollTop = position.current.top + node.scrollHeight - position.current.height;
      position.current = null;
    } else if (follow.current) node.scrollTop = node.scrollHeight;
  }, [messages]);

  async function more() {
    setLoadingMore(true);
    try {
      const page = await Desktop.messages(account.id, space.name, cursor, thread);
      if (!alive.current) return;
      if (scroller.current)
        position.current = {
          top: scroller.current.scrollTop,
          height: scroller.current.scrollHeight,
        };
      older.current = true;
      setMessages((previous) => mergeMessages(previous, page.messages ?? [], false));
      setCursor(page.nextPageToken);
    } catch (err) {
      if (alive.current) setError(describe(err));
    } finally {
      if (alive.current) setLoadingMore(false);
    }
  }

  return (
    <div className="timeline">
      {error && (
        <div className="message-notice">
          <Notice>{error}</Notice>
          <Button size="small" onClick={() => setRefresh((n) => n + 1)}>
            Tentar novamente
          </Button>
        </div>
      )}
      <section
        className="messages"
        ref={scroller}
        aria-label={thread ? "Mensagens da thread" : "Mensagens"}
        aria-busy={loading}
        onScroll={(event) => {
          const n = event.currentTarget;
          follow.current = n.scrollHeight - n.scrollTop - n.clientHeight < 100;
        }}
      >
        {cursor && (
          <div className="older-messages">
            <Button disabled={loadingMore} size="small" onClick={() => void more()}>
              {loadingMore ? "Carregando…" : "Carregar mensagens anteriores"}
            </Button>
          </div>
        )}
        {loading ? (
          <div className="center-state">
            <Spinner size="small" label="Carregando mensagens" />
          </div>
        ) : messages.length === 0 && !error ? (
          <div className="center-state">
            <h3>{thread ? "A thread ainda não tem mensagens" : "A conversa começa aqui"}</h3>
            <p>Escreva uma mensagem abaixo.</p>
          </div>
        ) : (
          messages.map((m, index) => (
            <MessageRow
              key={m.name}
              message={m}
              account={account}
              showDate={index === 0 || m.createTime.slice(0, 10) !== messages[index - 1].createTime.slice(0, 10)}
              onThread={
                onThread && space.spaceThreadingState === "THREADED_MESSAGES" && m.thread.name
                  ? () => onThread(m.thread.name)
                  : undefined
              }
            />
          ))
        )}
      </section>
      <Composer
        key={`${account.id}|${space.name}|${thread}`}
        account={account}
        space={space}
        thread={thread}
        onSent={(message) => {
          follow.current = true;
          setMessages((old) => mergeMessages(old, [message], false));
          setRefresh((n) => n + 1);
        }}
      />
    </div>
  );
}

function MessageRow({
  message: m,
  account,
  showDate,
  onThread,
}: {
  message: Message;
  account: Account;
  showDate: boolean;
  onThread?: () => void;
}) {
  const own = m.sender.name === `users/${account.id}`;
  const name = senderName(m, account);
  const date = new Date(m.createTime);
  return (
    <>
      {showDate && (
        <div className="date-divider">
          <time dateTime={m.createTime}>
            {Number.isNaN(date.valueOf())
              ? ""
              : date.toLocaleDateString("pt-BR", {
                  day: "numeric",
                  month: "long",
                  year: "numeric",
                })}
          </time>
        </div>
      )}
      <article className={`message ${own ? "own" : ""}`}>
        <Avatar aria-hidden name={name} color="neutral" size={32} />
        <div className="message-content">
          <header>
            <strong>{name}</strong>
            <time dateTime={m.createTime}>
              {Number.isNaN(date.valueOf())
                ? ""
                : date.toLocaleTimeString("pt-BR", {
                    hour: "2-digit",
                    minute: "2-digit",
                  })}
            </time>
            {m.threadReply && <span className="reply-label">Na thread</span>}
          </header>
          {m.text ? (
            <p className="message-text">
              {m.text.split(linkPattern).map((part, i) =>
                /^https?:\/\//.test(part) ? (
                  <a
                    // biome-ignore lint/suspicious/noArrayIndexKey: os pedaços vêm de um split fixo do texto
                    key={i}
                    href={part}
                    onClick={(event) => {
                      event.preventDefault();
                      void openLink(part);
                    }}
                  >
                    {part}
                  </a>
                ) : (
                  part
                ),
              )}
            </p>
          ) : (
            <p className="unsupported-content">Conteúdo disponível no Google Chat</p>
          )}
          {m.attachment?.map((file, i) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: anexos não têm identificador próprio na resposta
            <div className="attachment-label" key={i}>
              Anexo: {file.contentName || "Arquivo"}
            </div>
          ))}
          {onThread && (
            <Button
              className="thread-link"
              appearance="subtle"
              size="small"
              icon={<Chat24Regular />}
              onClick={onThread}
            >
              Abrir thread
            </Button>
          )}
        </div>
      </article>
    </>
  );
}

function Composer({
  account,
  space,
  thread,
  onSent,
}: {
  account: Account;
  space: Space;
  thread: string;
  onSent: (message: Message) => void;
}) {
  const key = `${account.id}|${space.name}|${thread}`;
  const [draft, setDraft] = useState(drafts.get(key)?.text ?? "");
  const [sending, setSending] = useState(false);
  const [error, setError] = useState("");
  const input = useRef<HTMLTextAreaElement>(null);
  const sendingRef = useRef(false);
  function change(text: string) {
    setDraft(text);
    drafts.set(key, { text, requestId: crypto.randomUUID() });
  }
  async function send() {
    if (!draft.trim() || sendingRef.current) return;
    sendingRef.current = true;
    setSending(true);
    setError("");
    const attempt = drafts.get(key) ?? {
      text: draft,
      requestId: crypto.randomUUID(),
    };
    drafts.set(key, attempt);
    try {
      const message = await Desktop.sendMessage(account.id, space.name, attempt.text, thread, attempt.requestId);
      drafts.delete(key);
      setDraft("");
      onSent(message);
      requestAnimationFrame(() => input.current?.focus());
    } catch (err) {
      setError(describe(err));
    } finally {
      sendingRef.current = false;
      setSending(false);
    }
  }
  return (
    <div className="composer-wrapper">
      {error && <Notice>{error} O texto foi preservado; você pode tentar enviar novamente.</Notice>}
      <form
        className="composer"
        onSubmit={(event) => {
          event.preventDefault();
          void send();
        }}
      >
        <label className="sr-only" htmlFor={`message-${thread ? "thread" : "main"}`}>
          {thread ? "Responder à thread" : "Escrever mensagem"}
        </label>
        <textarea
          ref={input}
          id={`message-${thread ? "thread" : "main"}`}
          value={draft}
          disabled={sending}
          placeholder={thread ? "Responder à thread…" : "Escrever uma mensagem…"}
          rows={2}
          maxLength={32000}
          onChange={(event) => change(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter" && !event.shiftKey && !event.nativeEvent.isComposing) {
              event.preventDefault();
              void send();
            }
          }}
        />
        <div className="composer-toolbar">
          <span className="sending-as" title={account.email}>
            {account.email}
          </span>
          <Button
            appearance="primary"
            icon={<Send24Regular />}
            type="submit"
            disabled={sending || !draft.trim()}
            aria-label={thread ? "Enviar resposta à thread" : "Enviar mensagem"}
          >
            {sending ? "Enviando…" : "Enviar"}
          </Button>
        </div>
      </form>
      <span className="composer-hint">Enter para enviar · Shift + Enter para nova linha</span>
    </div>
  );
}
