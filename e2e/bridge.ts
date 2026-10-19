import type { Page } from "@playwright/test";

// Ponte MyGo falsa para os testes de interface. Roda dentro da página, no lugar
// do runtime que o executável injeta, e guarda cada chamada em window.bridge.

export type Scenario = {
  configured?: boolean;
  accounts?: number;
  // atraso em ms por "Método" ou "Método:primeiro-argumento"
  delays?: Record<string, number>;
  // mensagem de erro por "Método" ou "Método:primeiro-argumento"; vale para a próxima chamada
  failOnce?: Record<string, string>;
};

export type Call = { method: string; args: unknown[] };

declare global {
  interface Window {
    bridge: { calls: Call[]; failOnce: Record<string, string>; state: { activeAccountId: string } };
  }
}

function install(scenario: Scenario) {
  const accounts = [
    { id: "111", name: "Ana Souza", email: "ana@empresa-a.com", domain: "empresa-a.com" },
    { id: "222", name: "Ana Souza", email: "ana@consultoria-b.com.br", domain: "consultoria-b.com.br" },
  ].slice(0, scenario.accounts ?? 2);
  let state = {
    configured: scenario.configured ?? true,
    clientId: scenario.configured === false ? "" : "123-abc.apps.googleusercontent.com",
    accounts,
    activeAccountId: accounts[0]?.id ?? "",
  };
  const iso = (minutes: number) => new Date(Date.UTC(2026, 9, 19, 12, 0) - minutes * 60000).toISOString();
  const spaces = (account: string) => [
    {
      name: `spaces/team${account}`,
      displayName: "Time de Plataforma",
      spaceType: "SPACE",
      spaceThreadingState: "THREADED_MESSAGES",
      lastActiveTime: iso(1),
      spaceUri: "https://chat.google.com/room/team",
    },
    {
      name: `spaces/dm${account}`,
      displayName: "Bruno Lima",
      spaceType: "DIRECT_MESSAGE",
      spaceThreadingState: "UNTHREADED_MESSAGES",
      lastActiveTime: iso(30),
      spaceUri: "",
    },
  ];
  const message = (space: string, n: number, text: string, sender: string) => ({
    name: `${space}/messages/m${n}`,
    text,
    createTime: iso(60 - n),
    sender: {
      name: `users/${sender}`,
      displayName: sender === "999" ? "Bruno Lima" : "",
      email: "",
      type: "HUMAN",
      isAnonymous: false,
    },
    thread: { name: `${space}/threads/t${n}` },
    threadReply: false,
  });
  const bridge = {
    calls: [] as { method: string; args: unknown[] }[],
    failOnce: { ...(scenario.failOnce ?? {}) },
    state,
  };
  let sent = 0;
  const posted: ReturnType<typeof message>[] = [];
  (globalThis as unknown as { bridge: typeof bridge }).bridge = bridge;
  (globalThis as unknown as { mygo: unknown }).mygo = {
    call: async (method: string, ...args: unknown[]) => {
      const name = method.replace("Desktop.", "");
      bridge.calls.push({ method: name, args });
      const key = `${name}:${String(args[0] ?? "")}`;
      const delay = scenario.delays?.[key] ?? scenario.delays?.[name] ?? 0;
      if (delay) await new Promise((resolve) => setTimeout(resolve, delay));
      const failure = bridge.failOnce[key] ?? bridge.failOnce[name];
      if (failure) {
        delete bridge.failOnce[key];
        delete bridge.failOnce[name];
        throw new Error(failure);
      }
      switch (name) {
        case "State":
          return state;
        case "SelectAccount":
          state = { ...state, activeAccountId: String(args[0]) };
          bridge.state = state;
          return state;
        case "Spaces":
          return { spaces: spaces(String(args[0])), nextPageToken: "" };
        case "Messages": {
          const space = String(args[1]);
          const thread = String(args[3]);
          const mine = posted.filter((m) => m.name.startsWith(`${space}/`) && (!thread || m.thread.name === thread));
          if (thread) return { messages: [message(space, 1, "Resposta na thread", "999"), ...mine], nextPageToken: "" };
          return {
            messages: [
              message(space, 1, "Bom dia, o deploy passou.", "999"),
              message(space, 2, "Detalhes em https://exemplo.com/tickets/42.", "111"),
              ...mine,
            ],
            nextPageToken: "",
          };
        }
        case "SendMessage":
          sent += 1;
          posted.push({
            ...message(String(args[1]), 40 + sent, String(args[2]), String(args[0])),
            thread: { name: String(args[3]) || `${args[1]}/threads/novo${sent}` },
            threadReply: Boolean(args[3]),
          });
          return posted.at(-1);
        default:
          return state;
      }
    },
  };
}

export async function withBridge(page: Page, scenario: Scenario = {}) {
  await page.addInitScript(install, scenario);
}

export function calls(page: Page, method?: string) {
  return page.evaluate((m) => window.bridge.calls.filter((c) => !m || c.method === m), method);
}
