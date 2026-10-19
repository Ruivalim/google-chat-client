import {
  Avatar,
  type BrandVariants,
  Button,
  createDarkTheme,
  createLightTheme,
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
  Switch,
  Tooltip,
} from "@fluentui/react-components";
import {
  Add24Regular,
  ArrowClockwise24Regular,
  ArrowDownload24Regular,
  ArrowLeft24Regular,
  ArrowUpload24Regular,
  Attach24Regular,
  Chat24Regular,
  Copy24Regular,
  Dismiss24Regular,
  Document24Regular,
  Open24Regular,
  People24Regular,
  QuestionCircle24Regular,
  Search24Regular,
  Send24Regular,
  Settings24Regular,
  WeatherMoon24Regular,
  WeatherSunny24Regular,
} from "@fluentui/react-icons";
import { isMyGo, onLocaleChange } from "mygo-runtime";
import { type ReactElement, type ReactNode, useEffect, useLayoutEffect, useRef, useState } from "react";
import logo from "./assets/logo.png";
import { chosenLanguage, currentLocale, setChosenLanguage, t } from "./i18n";
import {
  type Account,
  type Activity,
  type Attachment,
  type AttachmentDataRef,
  Desktop,
  events,
  type Message,
  type Notifications,
  type PickedFile,
  type Project,
  type Space,
  type State,
  type Unread,
} from "./mygo";

// Escala do verde de acento, do mais escuro (10) ao mais claro (160). O Fluent
// tira dela todas as cores de marca, inclusive estados como pressionado e
// selecionado; sem ela, sobram azuis do tema padrão.
const brand: BrandVariants = {
  10: "#011a12",
  20: "#01261b",
  30: "#013324",
  40: "#01402d",
  50: "#004d37",
  60: "#005a40",
  70: "#006b4c",
  80: "#007a57",
  90: "#1a8a66",
  100: "#339a77",
  110: "#4daa88",
  120: "#66b99a",
  130: "#80c8ac",
  140: "#99d6bf",
  150: "#b3e3d2",
  160: "#cdefe4",
};
const baseDark = createDarkTheme(brand);
const baseLight = createLightTheme(brand);

const emptyState: State = {
  projects: [],
  accounts: [],
  activeAccountId: "",
  notifications: { on: true, every: 60 },
  language: "",
};

// Os mesmos valores de chat.CheckIntervals; o Go recusa qualquer outro.
const checkIntervals: [number, () => string][] = [
  [15, () => t().interval15()],
  [30, () => t().interval30()],
  [60, () => t().interval60()],
  [120, () => t().interval120()],
  [300, () => t().interval300()],
];

// Idiomas oferecidos, cada um no próprio idioma.
const languageOptions: [string, string][] = [
  ["pt-BR", "Português (Brasil)"],
  ["en", "English"],
];

// Negrito (**assim**) e código (`assim`) dentro das mensagens do catálogo, que
// não carregam JSX: cada parágrafo fica uma frase inteira para quem traduz.
function rich(text: string): ReactNode[] {
  return text.split(/(\*\*[^*]+\*\*|`[^`]+`)/g).map((part, i) => {
    // biome-ignore lint/suspicious/noArrayIndexKey: os pedaços vêm de um split fixo do texto
    if (part.startsWith("**")) return <strong key={i}>{part.slice(2, -2)}</strong>;
    // biome-ignore lint/suspicious/noArrayIndexKey: os pedaços vêm de um split fixo do texto
    if (part.startsWith("`")) return <code key={i}>{part.slice(1, -1)}</code>;
    return part;
  });
}

type OpenRequest = { activity: Activity; n: number };
const native = isMyGo();
// Rascunho por conta, conversa e thread. Cada arquivo guarda a referência do
// upload e se já virou mensagem: o reenvio continua de onde parou, sem subir de
// novo nem duplicar mensagem (o requestId de cada uma é fixo).
type DraftFile = PickedFile & { requestId: string; ref?: AttachmentDataRef; sent?: boolean };
type Draft = { text: string; requestId: string; textSent?: boolean; files: DraftFile[] };
const drafts = new Map<string, Draft>();
const selectedSpaces = new Map<string, Space>();
type Appearance = "system" | "light" | "dark";

// As mensagens vêm do Go em minúscula, como manda a convenção de lá.
function describe(error: unknown) {
  const text = error instanceof Error ? error.message : String(error);
  return text.charAt(0).toLocaleUpperCase(currentLocale()) + text.slice(1);
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
      initials={initials ? initials.toLocaleUpperCase(currentLocale()) : undefined}
      color="colorful"
      idForColor={account.domain || account.email}
      size={size}
    />
  );
}

// O project_id vem no JSON do Google Cloud; sem ele, o número do projeto, que
// abre o client ID.
function projectLabel(project: Project) {
  return project.projectId || t().projectFallback(project.clientId.split("-")[0]);
}

function senderName(m: Message, account: Account) {
  if (m.sender.name === `users/${account.id}`) return account.name || account.email;
  if (m.sender.displayName) return m.sender.displayName;
  if (m.sender.email) return m.sender.email;
  if (m.sender.type === "BOT") return t().senderApp();
  return m.sender.isAnonymous ? t().senderUnavailable() : t().senderNoName();
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

// O login abre no navegador padrão. O link copiado serve para fazer o login em
// outro navegador ou container, por exemplo o da empresa da conta. Só o começo
// do fluxo pode ser copiado: as etapas seguintes dependem dos cookies da aba.
function LoginPending({ size }: { size?: "small" }) {
  const [url, setUrl] = useState("");
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    let stop = false;
    let timer: ReturnType<typeof setTimeout>;
    const poll = () =>
      Desktop.loginURL()
        .then((next) => {
          if (stop) return;
          setUrl(next);
          if (!next) timer = setTimeout(poll, 250);
        })
        .catch(() => undefined);
    void poll();
    return () => {
      stop = true;
      clearTimeout(timer);
    };
  }, []);
  useEffect(() => {
    if (!copied) return;
    const timer = setTimeout(() => setCopied(false), 2500);
    return () => clearTimeout(timer);
  }, [copied]);
  return (
    <>
      <span className="login-pending-text" role="status">
        <Spinner size="tiny" /> {t().loginPending()}
      </span>
      <Button
        size={size}
        icon={<Copy24Regular />}
        disabled={!url}
        onClick={() =>
          void Desktop.copyLoginURL()
            .then(() => setCopied(true))
            .catch(() => undefined)
        }
      >
        {copied ? t().linkCopied() : t().copyLink()}
      </Button>
      <Button size={size} onClick={() => void Desktop.cancelLogin()}>
        {t().cancel()}
      </Button>
    </>
  );
}

const scopes = [
  "openid",
  "email",
  "profile",
  "https://www.googleapis.com/auth/chat.spaces.readonly",
  "https://www.googleapis.com/auth/chat.memberships.readonly",
  "https://www.googleapis.com/auth/chat.messages",
];

const consoleURL = "https://console.cloud.google.com";

function GuideLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <Button size="small" appearance="subtle" icon={<Open24Regular />} onClick={() => void openLink(href)}>
      {children}
    </Button>
  );
}

// Passo a passo do Google Cloud. O que está aqui foi conferido na documentação
// do Google em outubro de 2026; o README traz as fontes.
function ConnectGuide({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) return;
    const timer = setTimeout(() => setCopied(false), 2500);
    return () => clearTimeout(timer);
  }, [copied]);
  function copyScopes() {
    const text = scopes.join("\n");
    const done = () => setCopied(true);
    if (native)
      void Desktop.copyText(text)
        .then(done)
        .catch(() => undefined);
    else
      void navigator.clipboard
        ?.writeText(text)
        .then(done)
        .catch(() => undefined);
  }
  return (
    <Dialog open={open} onOpenChange={(_, data) => !data.open && onClose()}>
      <DialogSurface className="settings-dialog guide-dialog">
        <DialogBody>
          <DialogTitle
            action={
              <IconButton label={t().guideClose()} appearance="subtle" onClick={onClose}>
                <Dismiss24Regular />
              </IconButton>
            }
          >
            {t().guideTitle()}
          </DialogTitle>
          <DialogContent>
            <ol className="guide">
              <li>
                <h2>{t().guideStep1Title()}</h2>
                <p>{rich(t().guideStep1Body())}</p>
                <GuideLink href={`${consoleURL}/projectcreate`}>{t().guideCreateProject()}</GuideLink>
              </li>
              <li>
                <h2>{t().guideStep2Title()}</h2>
                <p>{rich(t().guideStep2Body())}</p>
                <div className="guide-links">
                  <GuideLink href={`${consoleURL}/apis/library/chat.googleapis.com`}>{t().guideEnableApi()}</GuideLink>
                  <GuideLink href={`${consoleURL}/apis/api/chat.googleapis.com/hangouts-chat`}>
                    {t().guideConfiguration()}
                  </GuideLink>
                </div>
              </li>
              <li>
                <h2>{t().guideStep3Title()}</h2>
                <ul>
                  <li>{rich(t().guideBranding())}</li>
                  <li>{rich(t().guideAudience())}</li>
                  <li>{rich(t().guideDataAccess())}</li>
                </ul>
                <code className="guide-scopes">{scopes.join("\n")}</code>
                <div className="guide-links">
                  <Button size="small" icon={<Copy24Regular />} onClick={copyScopes}>
                    {copied ? t().scopesCopied() : t().copyScopes()}
                  </Button>
                  <GuideLink href={`${consoleURL}/auth/branding`}>{t().linkBranding()}</GuideLink>
                  <GuideLink href={`${consoleURL}/auth/audience`}>{t().linkAudience()}</GuideLink>
                  <GuideLink href={`${consoleURL}/auth/scopes`}>{t().linkDataAccess()}</GuideLink>
                </div>
              </li>
              <li>
                <h2>{t().guideStep4Title()}</h2>
                <ul>
                  <li>{rich(t().guideTesting())}</li>
                  <li>{rich(t().guideProduction())}</li>
                </ul>
              </li>
              <li>
                <h2>{t().guideStep5Title()}</h2>
                <p>{rich(t().guideStep5Body())}</p>
                <GuideLink href={`${consoleURL}/auth/clients`}>{t().linkClients()}</GuideLink>
              </li>
              <li>
                <h2>{t().guideStep6Title()}</h2>
                <p>{rich(t().guideStep6Body())}</p>
              </li>
            </ol>
          </DialogContent>
          <DialogActions>
            <Button appearance="primary" onClick={onClose}>
              {t().gotIt()}
            </Button>
          </DialogActions>
        </DialogBody>
      </DialogSurface>
    </Dialog>
  );
}

function Notice({ children }: { children: ReactNode }) {
  return (
    <MessageBar intent="error" layout="multiline">
      <MessageBarBody className="notice-body">
        {typeof children === "string" ? linkify(children) : children}
      </MessageBarBody>
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
  const [guide, setGuide] = useState(false);
  const [unread, setUnread] = useState<Unread[]>([]);
  const [openRequest, setOpenRequest] = useState<OpenRequest | null>(null);
  const [connecting, setConnecting] = useState(false);
  const [importing, setImporting] = useState(false);
  const [removing, setRemoving] = useState("");
  const [language, setLanguage] = useState(chosenLanguage());
  // Só para redesenhar quando o idioma do sistema ou do app muda.
  const [, setLocaleTick] = useState(0);
  const [appearance, setAppearance] = useState<Appearance>(() => {
    const saved = localStorage.getItem("appearance");
    return saved === "light" || saved === "dark" ? saved : "system";
  });
  const [systemDark, setSystemDark] = useState(matchMedia("(prefers-color-scheme: dark)").matches);
  const dark = appearance === "system" ? systemDark : appearance === "dark";
  const account = state.accounts.find((a) => a.id === state.activeAccountId) ?? state.accounts[0];
  const theme = dark
    ? {
        ...baseDark,
        colorBrandBackground: "#006b4c",
        colorBrandBackgroundHover: "#00845e",
        colorBrandForeground1: "#77dfb5",
        colorBrandForeground2: "#a2edd0",
        colorBrandStroke1: "#77dfb5",
        colorCompoundBrandBackground: "#23875f",
        colorCompoundBrandBackgroundHover: "#2a9d72",
        colorCompoundBrandBackgroundPressed: "#1c7350",
        colorCompoundBrandStroke: "#3fbf8f",
        colorCompoundBrandStrokeHover: "#5fd0a5",
        colorCompoundBrandForeground1: "#77dfb5",
      }
    : {
        ...baseLight,
        colorBrandBackground: "#006b4c",
        colorBrandBackgroundHover: "#00513a",
        colorBrandBackgroundPressed: "#003b2a",
        colorBrandForeground1: "#006b4c",
        colorBrandForeground2: "#00513a",
        colorBrandStroke1: "#006b4c",
        colorCompoundBrandBackground: "#006b4c",
        colorCompoundBrandBackgroundHover: "#00513a",
        colorCompoundBrandBackgroundPressed: "#003b2a",
        colorCompoundBrandStroke: "#006b4c",
        colorCompoundBrandStrokeHover: "#00513a",
        colorCompoundBrandForeground1: "#006b4c",
      };

  // biome-ignore lint/correctness/useExhaustiveDependencies: select só usa refs e setState funcional
  useEffect(() => {
    if (!native) return;
    Desktop.unread()
      .then(setUnread)
      .catch(() => undefined);
    const offUnread = events.unreadChanged.on(setUnread);
    // Clique numa notificação: troca para a conta e abre a conversa.
    const offOpen = events.openConversation.on((activity) => {
      setSettings(false);
      setGuide(false);
      void select(activity.accountId);
      setOpenRequest((current) => ({ activity, n: (current?.n ?? 0) + 1 }));
    });
    return () => {
      offUnread();
      offOpen();
    };
  }, []);
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
    if (!native) return;
    return onLocaleChange(() => setLocaleTick((n) => n + 1));
  }, []);
  useEffect(() => {
    document.documentElement.lang = currentLocale();
  });
  // No desktop o idioma salvo vem do Go; a página começa seguindo o que o MyGo
  // já aplicou e só precisa saber qual opção marcar.
  useEffect(() => {
    if (native) setLanguage(state.language);
  }, [state.language]);
  useEffect(() => {
    document.documentElement.dataset.theme = dark ? "dark" : "light";
    localStorage.setItem("appearance", appearance);
  }, [appearance, dark]);

  // Sem projeto escolhido e com mais de um importado, quem decide é a pessoa,
  // na lista de projetos das configurações.
  async function connect(clientId?: string) {
    const target = clientId ?? (state.projects.length === 1 ? state.projects[0].clientId : "");
    if (!target) {
      setSettings(true);
      return;
    }
    setError("");
    setConnecting(true);
    try {
      const next = await Desktop.connectAccount(target);
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

  async function chooseLanguage(tag: string) {
    setError("");
    setChosenLanguage(tag);
    setLanguage(tag);
    if (!native) return;
    try {
      setState(await Desktop.setLanguage(tag));
    } catch (err) {
      setError(describe(err));
    }
  }

  async function saveNotifications(next: Notifications) {
    setError("");
    try {
      setState(await Desktop.setNotifications(next));
    } catch (err) {
      setError(describe(err));
    }
  }

  async function removeProject(clientId: string) {
    setRemoving(clientId);
    setError("");
    try {
      setState(await Desktop.removeProject(clientId));
    } catch (err) {
      setError(describe(err));
    } finally {
      setRemoving("");
    }
  }

  const configured = state.projects.length > 0;
  const unreadOf = (accountId: string) =>
    unread.filter((u) => u.accountId === accountId).reduce((sum, u) => sum + u.count, 0);
  const manyProjects = state.projects.length > 1;
  const accountsOf = (clientId: string) => state.accounts.filter((a) => a.clientId === clientId).length;
  const projectOf = (a: Account) => state.projects.find((p) => p.clientId === a.clientId);

  // A className do FluentProvider vai também para o contêiner dos portais (tooltip,
  // diálogo). Com a classe de layout ali, esse contêiner cobria a tela e o tooltip piscava.
  return (
    <FluentProvider theme={theme}>
      <div className="app">
        <a href="#main-content" className="skip-link">
          {t().skipToConversation()}
        </a>
        <nav className="account-rail" aria-label={t().googleAccounts()}>
          <div className="rail-heading" title="Google Chat Client">
            <img src={logo} alt="" width={32} height={32} />
          </div>
          <div className="account-buttons">
            {state.accounts.map((a) => (
              <Tooltip key={a.id} content={`${a.name || a.email} · ${a.email}`} relationship="label">
                <button
                  type="button"
                  className={`account-button ${account?.id === a.id ? "selected" : ""}`}
                  aria-pressed={account?.id === a.id}
                  aria-label={t().switchToAccount(unreadOf(a.id), a.email)}
                  onClick={() => void select(a.id)}
                >
                  <AccountAvatar account={a} size={36} />
                  {unreadOf(a.id) > 0 && (
                    <span className="unread-badge" aria-hidden>
                      {unreadOf(a.id) > 99 ? "99+" : unreadOf(a.id)}
                    </span>
                  )}
                </button>
              </Tooltip>
            ))}
          </div>
          <IconButton
            label={t().addGoogleAccount()}
            appearance="subtle"
            disabled={!native || connecting}
            onClick={() => void connect()}
          >
            <Add24Regular />
          </IconButton>
          <div className="rail-bottom">
            <IconButton
              label={dark ? t().lightThemeAction() : t().darkThemeAction()}
              appearance="subtle"
              onClick={() => setAppearance(dark ? "light" : "dark")}
            >
              {dark ? <WeatherSunny24Regular /> : <WeatherMoon24Regular />}
            </IconButton>
            <IconButton label={t().settingsAndAccounts()} appearance="subtle" onClick={() => setSettings(true)}>
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
            <div className="connection-status">
              <LoginPending size="small" />
            </div>
          )}
          {starting ? (
            <div className="center-state">
              <Spinner label={t().loadingAccounts()} />
            </div>
          ) : account ? (
            <Workspace
              key={account.id}
              account={account}
              unread={unread}
              openRequest={openRequest?.activity.accountId === account.id ? openRequest : null}
              onSettings={() => setSettings(true)}
            />
          ) : (
            <div className="welcome">
              <div className="welcome-heading">
                <img src={logo} alt="" width={28} height={28} />
                <span>Google Chat Client</span>
              </div>
              <div className="welcome-content">
                <p className="eyebrow">{t().welcomeEyebrow()}</p>
                <h1>
                  {t().welcomeTitle1()}
                  <br />
                  {t().welcomeTitle2()}
                </h1>
                <p className="welcome-description">{t().welcomeDescription()}</p>
                {!native && <div className="preview-note">{t().previewNote()}</div>}
                <div className="welcome-action">
                  {configured ? (
                    <Button
                      appearance="primary"
                      size="large"
                      icon={<Add24Regular />}
                      disabled={!native || connecting}
                      onClick={() => void connect()}
                    >
                      {t().connectGoogleAccount()}
                    </Button>
                  ) : (
                    <Button
                      appearance="primary"
                      size="large"
                      icon={<ArrowUpload24Regular />}
                      disabled={!native || importing}
                      onClick={() => void importCredentials()}
                    >
                      {t().importGoogleCredentials()}
                    </Button>
                  )}
                  <Button appearance="subtle" onClick={() => setGuide(true)}>
                    {t().howToConnect()}
                  </Button>
                </div>
                <p className="welcome-detail">
                  {!configured
                    ? t().welcomeNoProject()
                    : manyProjects
                      ? t().welcomeManyProjects(state.projects.length)
                      : t().welcomeOneProject()}
                </p>
              </div>
              <div className="welcome-footer">
                <span>{t().welcomeTagline()}</span>
                <span>Windows · Linux · macOS</span>
              </div>
            </div>
          )}
        </main>
        <ConnectGuide open={guide} onClose={() => setGuide(false)} />
        <Dialog open={settings} onOpenChange={(_, data) => setSettings(data.open)}>
          <DialogSurface className="settings-dialog">
            <DialogBody>
              <DialogTitle
                action={
                  <IconButton label={t().closeSettings()} appearance="subtle" onClick={() => setSettings(false)}>
                    <Dismiss24Regular />
                  </IconButton>
                }
              >
                {t().settingsTitle()}
              </DialogTitle>
              <DialogContent>
                {error && <Notice>{error}</Notice>}
                <section className="settings-section">
                  <h2>{t().googleAccounts()}</h2>
                  <p>{t().accountsDescription()}</p>
                  {state.accounts.map((a) => (
                    <div className="settings-account" key={a.id}>
                      <AccountAvatar account={a} />
                      <div>
                        <strong>{a.name || a.email}</strong>
                        <span>
                          {a.email}
                          {manyProjects &&
                            ` · ${projectOf(a) ? projectLabel(projectOf(a) as Project) : t().projectRemoved()}`}
                        </span>
                      </div>
                      <Button disabled={removing !== "" || connecting} onClick={() => void remove(a.id)}>
                        {removing === a.id ? t().removing() : t().remove()}
                      </Button>
                    </div>
                  ))}
                  {connecting ? (
                    <div className="login-pending">
                      <LoginPending />
                    </div>
                  ) : !configured ? (
                    <p className="settings-hint">{t().hintNoProject()}</p>
                  ) : manyProjects ? (
                    <p className="settings-hint">{t().hintManyProjects()}</p>
                  ) : (
                    <Button icon={<Add24Regular />} disabled={!native || importing} onClick={() => void connect()}>
                      {t().addGoogleAccount()}
                    </Button>
                  )}
                </section>
                <section className="settings-section">
                  <h2>{t().projectsHeading()}</h2>
                  <p>{t().projectsDescription()}</p>
                  <Button
                    className="guide-open"
                    appearance="subtle"
                    icon={<QuestionCircle24Regular />}
                    onClick={() => {
                      setSettings(false);
                      setGuide(true);
                    }}
                  >
                    {t().openGuide()}
                  </Button>
                  {state.projects.map((p) => {
                    const count = accountsOf(p.clientId);
                    return (
                      <div className="settings-project" key={p.clientId}>
                        <div>
                          <strong>{projectLabel(p)}</strong>
                          <code className="client-id">{p.clientId}</code>
                          <span>{t().projectAccounts(count)}</span>
                        </div>
                        {manyProjects && (
                          <Button
                            icon={<Add24Regular />}
                            disabled={!native || connecting || importing}
                            onClick={() => void connect(p.clientId)}
                          >
                            {t().addAccount()}
                          </Button>
                        )}
                        <Tooltip
                          content={count > 0 ? t().removeProjectFirst() : t().removeProject()}
                          relationship="description"
                        >
                          <Button
                            disabledFocusable={count > 0}
                            disabled={removing !== "" || connecting}
                            onClick={() => void removeProject(p.clientId)}
                          >
                            {removing === p.clientId ? t().removing() : t().remove()}
                          </Button>
                        </Tooltip>
                      </div>
                    );
                  })}
                  <div className="settings-actions">
                    <Button
                      icon={<ArrowUpload24Regular />}
                      disabled={!native || importing || connecting}
                      onClick={() => void importCredentials()}
                    >
                      {importing ? t().importing() : configured ? t().importAnotherJson() : t().importJson()}
                    </Button>
                    <Button
                      appearance="subtle"
                      icon={<Open24Regular />}
                      onClick={() => void openLink("https://console.cloud.google.com/auth/clients")}
                    >
                      {t().openGoogleCloud()}
                    </Button>
                  </div>
                  <p>{t().projectsAdminNote()}</p>
                </section>
                <section className="settings-section">
                  <h2>{t().notificationsHeading()}</h2>
                  <p>{t().notificationsDescription()}</p>
                  <div className="notification-setting">
                    <Switch
                      label={t().notifyToggle()}
                      checked={state.notifications.on}
                      disabled={!native}
                      onChange={(_, data) => void saveNotifications({ ...state.notifications, on: data.checked })}
                    />
                    <label htmlFor="check-every">{t().checkEvery()}</label>
                    <Select
                      id="check-every"
                      value={String(state.notifications.every)}
                      disabled={!native || !state.notifications.on}
                      onChange={(_, data) =>
                        void saveNotifications({ ...state.notifications, every: Number(data.value) })
                      }
                    >
                      {checkIntervals.map(([seconds, label]) => (
                        <option key={seconds} value={seconds}>
                          {label()}
                        </option>
                      ))}
                    </Select>
                  </div>
                </section>
                <section className="settings-section appearance-setting">
                  <label htmlFor="language">{t().language()}</label>
                  <Select id="language" value={language} onChange={(_, data) => void chooseLanguage(data.value)}>
                    <option value="">{t().followSystem()}</option>
                    {languageOptions.map(([tag, name]) => (
                      <option key={tag} value={tag} lang={tag}>
                        {name}
                      </option>
                    ))}
                  </Select>
                </section>
                <section className="settings-section appearance-setting">
                  <label htmlFor="appearance">{t().appearance()}</label>
                  <Select
                    id="appearance"
                    value={appearance}
                    onChange={(_, data) => setAppearance(data.value as Appearance)}
                  >
                    <option value="system">{t().followSystem()}</option>
                    <option value="light">{t().light()}</option>
                    <option value="dark">{t().dark()}</option>
                  </Select>
                </section>
              </DialogContent>
              <DialogActions>
                <Button appearance="primary" onClick={() => setSettings(false)}>
                  {t().done()}
                </Button>
              </DialogActions>
            </DialogBody>
          </DialogSurface>
        </Dialog>
      </div>
    </FluentProvider>
  );
}

function Workspace({
  account,
  unread,
  openRequest,
  onSettings,
}: {
  account: Account;
  unread: Unread[];
  openRequest: OpenRequest | null;
  onSettings: () => void;
}) {
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
  // O Go não avisa da conversa que está na tela e zera as não lidas dela.
  useEffect(() => {
    if (!native) return;
    void Desktop.setOpenConversation(account.id, selected?.name ?? "");
    return () => void Desktop.setOpenConversation("", "");
  }, [account.id, selected?.name]);
  // biome-ignore lint/correctness/useExhaustiveDependencies: o pedido muda a cada clique em notificação (n)
  useEffect(() => {
    if (!openRequest) return;
    const space = openRequest.activity.space;
    // A conversa pode ainda não estar na página carregada da lista.
    setSpaces((old) => (old.some((s) => s.name === space.name) ? old : [space, ...old]));
    select(space);
  }, [openRequest?.n]);
  const unreadIn = (space: string) => unread.find((u) => u.accountId === account.id && u.space === space)?.count ?? 0;
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
      <aside className="conversations" aria-label={t().conversationsOfAccount()}>
        <header className="workspace-heading">
          <div>
            <h1>{t().chatHeading()}</h1>
            <button type="button" className="account-label" onClick={onSettings} title={account.email}>
              {account.domain || account.email}
            </button>
          </div>
          <IconButton
            label={t().refreshConversations()}
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
            placeholder={t().filterPlaceholder()}
            aria-label={t().filterLabel()}
            value={query}
            onChange={(_, data) => setQuery(data.value)}
          />
          <fieldset className="filter-tabs" aria-label={t().conversationType()}>
            {[
              ["all", t().filterAll()],
              ["chats", t().filterChats()],
              ["spaces", t().filterSpaces()],
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
            <Button onClick={() => setRefresh((n) => n + 1)}>{t().tryAgain()}</Button>
          </div>
        )}
        <div className="conversation-list" aria-busy={loading}>
          {loading && spaces.length === 0 ? (
            <Spinner size="small" label={t().searchingConversations()} />
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
                      ? t().typeSpace()
                      : s.spaceType === "GROUP_CHAT"
                        ? t().typeGroup()
                        : t().typeDirect()}
                  </span>
                </span>
                {unreadIn(s.name) > 0 && selected?.name !== s.name && (
                  <span className="unread-badge inline">
                    {unreadIn(s.name)}
                    <span className="sr-only"> {t().unreadSuffix()}</span>
                  </span>
                )}
              </button>
            ))
          ) : (
            !loading &&
            !error && (
              <div className="list-empty">{query || filter !== "all" ? t().noMatch() : t().noConversations()}</div>
            )
          )}
          {cursor && (
            <Button className="more-conversations" disabled={loadingMore} onClick={() => void more()}>
              {loadingMore ? t().loading() : t().moreConversations()}
            </Button>
          )}
        </div>
        <footer className="workspace-footer">
          <span title={account.email}>{account.email}</span>
          <span>{t().activeAccount()}</span>
        </footer>
      </aside>
      <section className="conversation" aria-label={t().conversationHistory()}>
        {selected ? (
          <>
            <header className="conversation-header">
              <IconButton
                label={t().backToConversations()}
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
                  label={t().openInGoogleChat()}
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
                <aside className="thread-panel" aria-label={t().thread()}>
                  <header>
                    <h3>{t().thread()}</h3>
                    <IconButton label={t().closeThread()} appearance="subtle" onClick={() => setThread("")}>
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
            <h2>{t().chooseTitle()}</h2>
            <p>{t().chooseBody()}</p>
            <span className="current-account">{t().usingAccount(account.email)}</span>
          </div>
        )}
      </section>
    </div>
  );
}

const linkPattern = /(https?:\/\/[^\s<>]*[^\s<>.,;:!?)\]'"])/g;

// Links abrem no navegador do sistema, nunca dentro da janela do app.
function linkify(text: string) {
  return text.split(linkPattern).map((part, i) =>
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
  );
}

function spaceName(space: Space) {
  const id = space.name.split("/").at(-1) ?? "";
  return space.displayName || (space.spaceType === "SPACE" ? t().spaceFallback(id) : t().conversationFallback(id));
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
            {t().tryAgain()}
          </Button>
        </div>
      )}
      <section
        className="messages"
        ref={scroller}
        aria-label={thread ? t().threadMessages() : t().messagesLabel()}
        aria-busy={loading}
        onScroll={(event) => {
          const n = event.currentTarget;
          follow.current = n.scrollHeight - n.scrollTop - n.clientHeight < 100;
        }}
      >
        {cursor && (
          <div className="older-messages">
            <Button disabled={loadingMore} size="small" onClick={() => void more()}>
              {loadingMore ? t().loading() : t().loadOlder()}
            </Button>
          </div>
        )}
        {loading ? (
          <div className="center-state">
            <Spinner size="small" label={t().loadingMessages()} />
          </div>
        ) : messages.length === 0 && !error ? (
          <div className="center-state">
            <h3>{thread ? t().threadEmpty() : t().conversationStarts()}</h3>
            <p>{t().writeBelow()}</p>
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
              : date.toLocaleDateString(currentLocale(), {
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
                : date.toLocaleTimeString(currentLocale(), {
                    hour: "2-digit",
                    minute: "2-digit",
                  })}
            </time>
            {m.threadReply && <span className="reply-label">{t().inThread()}</span>}
          </header>
          {m.text ? (
            <p className="message-text">{linkify(m.text)}</p>
          ) : (
            !m.attachment?.length && <p className="unsupported-content">{t().unsupported()}</p>
          )}
          {m.attachment?.map((file, i) => (
            <MessageAttachment key={file.name || i} account={account} file={file} />
          ))}
          {onThread && (
            <Button
              className="thread-link"
              appearance="subtle"
              size="small"
              icon={<Chat24Regular />}
              onClick={onThread}
            >
              {t().openThread()}
            </Button>
          )}
        </div>
      </article>
    </>
  );
}

// Arquivo enviado no Chat desce pela API; arquivo do Drive só abre no Drive.
function MessageAttachment({ account, file }: { account: Account; file: Attachment }) {
  const [state, setState] = useState<"idle" | "saving" | "error">("idle");
  const [saved, setSaved] = useState("");
  const name = file.contentName || t().fileFallback();
  const resource = file.attachmentDataRef?.resourceName ?? "";
  const drive = file.driveDataRef?.driveFileId ?? "";
  async function download() {
    setState("saving");
    try {
      const path = await Desktop.downloadAttachment(account.id, resource, name);
      if (path) setSaved(path);
      setState("idle");
    } catch {
      setState("error");
    }
  }
  return (
    <div className="attachment">
      <Document24Regular aria-hidden />
      <span className="file-name" title={name}>
        {name}
      </span>
      {drive ? (
        <Button
          size="small"
          appearance="subtle"
          icon={<Open24Regular />}
          onClick={() => void openLink(`https://drive.google.com/file/d/${encodeURIComponent(drive)}/view`)}
        >
          {t().openInDrive()}
        </Button>
      ) : resource && native ? (
        saved ? (
          <Button size="small" appearance="subtle" onClick={() => void Desktop.showInFolder(saved)}>
            {t().savedShowInFolder()}
          </Button>
        ) : (
          <Button
            size="small"
            appearance="subtle"
            icon={<ArrowDownload24Regular />}
            disabled={state === "saving"}
            aria-label={t().downloadFile(name)}
            onClick={() => void download()}
          >
            {state === "saving" ? t().downloading() : state === "error" ? t().downloadFailed() : t().download()}
          </Button>
        )
      ) : (
        file.downloadUri && (
          <Button
            size="small"
            appearance="subtle"
            icon={<Open24Regular />}
            onClick={() => void openLink(file.downloadUri)}
          >
            {t().openAttachmentInChat()}
          </Button>
        )
      )}
    </div>
  );
}

function formatSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${(bytes / 1024 / 1024).toLocaleString(currentLocale(), { maximumFractionDigits: 1 })} MB`;
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
  const saved = drafts.get(key);
  const [draft, setDraft] = useState(saved?.text ?? "");
  const [files, setFiles] = useState<DraftFile[]>(saved?.files ?? []);
  const [sending, setSending] = useState("");
  const [error, setError] = useState("");
  const input = useRef<HTMLTextAreaElement>(null);
  const sendingRef = useRef(false);
  const current = (): Draft => drafts.get(key) ?? { text: draft, requestId: crypto.randomUUID(), files };
  function store(next: Draft) {
    drafts.set(key, next);
    setFiles(next.files);
  }
  function change(text: string) {
    setDraft(text);
    // Texto novo é mensagem nova: outro requestId, para o Google não confundir.
    drafts.set(key, { ...current(), text, requestId: crypto.randomUUID(), textSent: false });
  }
  async function pick() {
    setError("");
    try {
      const picked = await Desktop.pickFiles();
      if (!picked.length) return;
      const base = current();
      store({ ...base, files: [...base.files, ...picked.map((f) => ({ ...f, requestId: crypto.randomUUID() }))] });
      requestAnimationFrame(() => input.current?.focus());
    } catch (err) {
      setError(describe(err));
    }
  }
  function unpick(id: string) {
    const base = current();
    store({ ...base, files: base.files.filter((f) => f.id !== id) });
    void Desktop.forgetFile(id);
  }
  const pending = files.filter((f) => !f.sent);
  const blocked = pending.some((f) => f.tooLarge);
  async function send() {
    if ((!draft.trim() && pending.length === 0) || blocked || sendingRef.current) return;
    sendingRef.current = true;
    setError("");
    let attempt = current();
    drafts.set(key, attempt);
    try {
      if (attempt.files.length === 0) {
        setSending(t().sending());
        onSent(await Desktop.sendMessage(account.id, space.name, attempt.text, thread, attempt.requestId, null));
      }
      // Um arquivo por mensagem; o texto vai junto com o primeiro.
      for (const file of attempt.files) {
        if (file.sent) continue;
        const update = (patch: Partial<DraftFile>, extra: Partial<Draft> = {}) => {
          attempt = {
            ...attempt,
            ...extra,
            files: attempt.files.map((f) => (f.id === file.id ? { ...f, ...patch } : f)),
          };
          store(attempt);
        };
        let ref = file.ref;
        if (!ref) {
          setSending(t().sendingFile(file.name));
          ref = await Desktop.uploadAttachment(account.id, space.name, file.id);
          update({ ref });
        }
        const withText = !attempt.textSent && attempt.text.trim() !== "";
        const message = await Desktop.sendMessage(
          account.id,
          space.name,
          withText ? attempt.text : "",
          thread,
          withText ? attempt.requestId : file.requestId,
          ref,
        );
        update({ sent: true }, withText ? { textSent: true } : {});
        if (withText) setDraft("");
        void Desktop.forgetFile(file.id);
        onSent(message);
      }
      drafts.delete(key);
      setDraft("");
      setFiles([]);
      requestAnimationFrame(() => input.current?.focus());
    } catch (err) {
      setError(describe(err));
    } finally {
      sendingRef.current = false;
      setSending("");
    }
  }
  return (
    <div className="composer-wrapper">
      {error && (
        <Notice>
          {error} {files.length ? t().keptFiles() : t().keptText()}
        </Notice>
      )}
      <form
        className="composer"
        onSubmit={(event) => {
          event.preventDefault();
          void send();
        }}
      >
        <label className="sr-only" htmlFor={`message-${thread ? "thread" : "main"}`}>
          {thread ? t().replyToThread() : t().writeMessage()}
        </label>
        {pending.length > 0 && (
          <ul className="composer-files" aria-label={t().filesToSend()}>
            {pending.map((f) => (
              <li key={f.id} className={`composer-file${f.tooLarge ? " too-large" : ""}`}>
                <Document24Regular aria-hidden />
                <span className="file-name">{f.name}</span>
                <span className="file-size">{f.tooLarge ? t().overLimit() : formatSize(f.size)}</span>
                <IconButton
                  label={t().removeFile(f.name)}
                  appearance="subtle"
                  disabled={sending !== ""}
                  onClick={() => unpick(f.id)}
                >
                  <Dismiss24Regular />
                </IconButton>
              </li>
            ))}
          </ul>
        )}
        <textarea
          ref={input}
          id={`message-${thread ? "thread" : "main"}`}
          value={draft}
          disabled={sending !== ""}
          placeholder={thread ? t().replyPlaceholder() : t().messagePlaceholder()}
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
          <IconButton
            label={t().attachFiles()}
            appearance="subtle"
            disabled={!native || sending !== ""}
            onClick={() => void pick()}
          >
            <Attach24Regular />
          </IconButton>
          <span className="sending-as" title={account.email}>
            {sending || account.email}
          </span>
          <Button
            appearance="primary"
            icon={<Send24Regular />}
            type="submit"
            disabled={sending !== "" || blocked || (!draft.trim() && pending.length === 0)}
            aria-label={thread ? t().sendReply() : t().sendMessage()}
          >
            {sending ? t().sending() : t().send()}
          </Button>
        </div>
      </form>
      <span className="composer-hint">{t().composerHint()}</span>
    </div>
  );
}
