import { expect, test } from "@playwright/test";
import { calls, withBridge } from "./bridge";

const account = (page: import("@playwright/test").Page, email: string) =>
  page.getByRole("navigation", { name: "Contas Google" }).getByRole("button", { name: `Usar ${email}` });

test("prévia no navegador não oferece login", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByText("Abra o aplicativo desktop para conectar suas contas.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Importar credenciais Google" })).toBeDisabled();
});

test("sem projeto configurado, o primeiro passo é importar o JSON", async ({ page }) => {
  await withBridge(page, { configured: false, accounts: 0 });
  await page.goto("/");
  await expect(page.getByRole("button", { name: "Importar credenciais Google" })).toBeEnabled();
  await expect(page.getByRole("button", { name: "Conectar conta Google" })).toHaveCount(0);
});

test("cliques rápidos na faixa de contas terminam na última conta clicada", async ({ page }) => {
  // A primeira troca demora; a segunda responde na hora. A resposta atrasada não pode vencer.
  await withBridge(page, { delays: { "SelectAccount:222": 600 } });
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Chat" })).toBeVisible();

  await account(page, "ana@consultoria-b.com.br").click();
  await account(page, "ana@empresa-a.com").click();

  await expect(account(page, "ana@empresa-a.com")).toHaveAttribute("aria-pressed", "true");
  await page.waitForTimeout(900);
  await expect(account(page, "ana@empresa-a.com")).toHaveAttribute("aria-pressed", "true");
  await expect(page.locator(".workspace-footer")).toContainText("ana@empresa-a.com");
  expect((await calls(page, "SelectAccount")).map((c) => c.args[0])).toEqual(["222", "111"]);
  expect(await page.evaluate(() => window.bridge.state.activeAccountId)).toBe("111");
});

test("falha ao trocar de conta volta ao estado real e mostra o erro", async ({ page }) => {
  await withBridge(page, { failOnce: { "SelectAccount:222": "o chaveiro está bloqueado" } });
  await page.goto("/");
  await account(page, "ana@consultoria-b.com.br").click();
  await expect(page.getByText("O chaveiro está bloqueado")).toBeVisible();
  await expect(account(page, "ana@empresa-a.com")).toHaveAttribute("aria-pressed", "true");
});

test("cada conta tem as próprias conversas", async ({ page }) => {
  await withBridge(page);
  await page.goto("/");
  await page.getByRole("button", { name: /Bruno Lima/ }).click();
  await account(page, "ana@consultoria-b.com.br").click();
  await expect(page.getByText("Uma conversa de cada vez")).toBeVisible();
  const spaces = await calls(page, "Spaces");
  expect(spaces.map((c) => c.args[0])).toEqual(["111", "222"]);
});

test("Enter envia, Shift+Enter quebra linha e composição de texto não envia", async ({ page }) => {
  await withBridge(page);
  await page.goto("/");
  await page.getByRole("button", { name: /Bruno Lima/ }).click();
  const box = page.getByRole("textbox", { name: "Escrever mensagem" });

  await box.fill("primeira linha");
  await box.press("Shift+Enter");
  await box.pressSequentially("segunda");
  // Enter durante composição (IME) confirma o caractere, não a mensagem.
  await box.evaluate((el) =>
    el.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", isComposing: true, bubbles: true })),
  );
  expect(await calls(page, "SendMessage")).toHaveLength(0);
  await expect(box).toHaveValue("primeira linha\nsegunda");

  await box.press("Enter");
  await expect(page.getByText("segunda", { exact: false }).first()).toBeVisible();
  const [sent] = await calls(page, "SendMessage");
  expect(sent.args.slice(0, 4)).toEqual(["111", "spaces/dm111", "primeira linha\nsegunda", ""]);
  await expect(box).toHaveValue("");
  await expect(box).toBeFocused();
});

test("mensagem só com espaços não é enviada", async ({ page }) => {
  await withBridge(page);
  await page.goto("/");
  await page.getByRole("button", { name: /Bruno Lima/ }).click();
  const box = page.getByRole("textbox", { name: "Escrever mensagem" });
  await box.fill("   \n  ");
  await box.press("Enter");
  await expect(page.getByRole("button", { name: "Enviar mensagem" })).toBeDisabled();
  expect(await calls(page, "SendMessage")).toHaveLength(0);
});

test("reenvio depois de falha usa o mesmo requestId e preserva o texto", async ({ page }) => {
  await withBridge(page, { failOnce: { SendMessage: "não foi possível alcançar o Google Chat" } });
  await page.goto("/");
  await page.getByRole("button", { name: /Bruno Lima/ }).click();
  const box = page.getByRole("textbox", { name: "Escrever mensagem" });
  await box.fill("mensagem importante");
  await box.press("Enter");

  await expect(page.getByText("O texto foi preservado")).toBeVisible();
  await expect(box).toHaveValue("mensagem importante");
  await box.press("Enter");
  await expect(box).toHaveValue("");

  const [first, retry] = await calls(page, "SendMessage");
  expect(retry.args[4]).toBe(first.args[4]);
  expect(String(first.args[4])).toMatch(/^[0-9a-f-]{36}$/);
});

test("editar o texto depois da falha gera outro requestId", async ({ page }) => {
  await withBridge(page, { failOnce: { SendMessage: "falhou" } });
  await page.goto("/");
  await page.getByRole("button", { name: /Bruno Lima/ }).click();
  const box = page.getByRole("textbox", { name: "Escrever mensagem" });
  await box.fill("versão um");
  await box.press("Enter");
  await expect(page.getByText("O texto foi preservado")).toBeVisible();
  await box.fill("versão dois");
  await box.press("Enter");
  const [first, second] = await calls(page, "SendMessage");
  expect(second.args[2]).toBe("versão dois");
  expect(second.args[4]).not.toBe(first.args[4]);
});

test("rascunho fica na conversa onde foi escrito", async ({ page }) => {
  await withBridge(page);
  await page.goto("/");
  await page.getByRole("button", { name: /Bruno Lima/ }).click();
  await page.getByRole("textbox", { name: "Escrever mensagem" }).fill("rascunho para o Bruno");
  await page.getByRole("button", { name: /Time de Plataforma/ }).click();
  await expect(page.getByRole("textbox", { name: "Escrever mensagem" })).toHaveValue("");
  await page.getByRole("button", { name: /Bruno Lima/ }).click();
  await expect(page.getByRole("textbox", { name: "Escrever mensagem" })).toHaveValue("rascunho para o Bruno");
});

test("resposta na thread vai para a thread certa", async ({ page }) => {
  await withBridge(page);
  await page.goto("/");
  await page.getByRole("button", { name: /Time de Plataforma/ }).click();
  await page.getByRole("button", { name: "Abrir thread" }).first().click();
  const thread = page.getByRole("complementary", { name: "Thread" });
  await expect(thread.getByText("Resposta na thread")).toBeVisible();
  await thread.getByRole("textbox", { name: "Responder à thread" }).fill("concordo");
  await thread.getByRole("button", { name: "Enviar resposta à thread" }).click();
  await expect(thread.getByText("concordo")).toBeVisible();
  const [sent] = await calls(page, "SendMessage");
  expect(sent.args.slice(1, 4)).toEqual(["spaces/team111", "concordo", "spaces/team111/threads/t1"]);
  await thread.getByRole("button", { name: "Fechar thread" }).click();
  await expect(thread).toHaveCount(0);
});

test("link na mensagem abre fora do app e sem a pontuação final", async ({ page }) => {
  await withBridge(page);
  await page.goto("/");
  await page.getByRole("button", { name: /Bruno Lima/ }).click();
  await page.getByRole("link", { name: "https://exemplo.com/tickets/42" }).click();
  await expect
    .poll(() => calls(page, "OpenLink"))
    .toEqual([{ method: "OpenLink", args: ["https://exemplo.com/tickets/42"] }]);
  expect(page.url()).toMatch(/127\.0\.0\.1:4173\/$/);
});

test("erro ao listar conversas oferece nova tentativa", async ({ page }) => {
  await withBridge(page, { failOnce: { Spaces: "não foi possível alcançar o Google Chat; confira a conexão" } });
  await page.goto("/");
  await expect(page.getByText("Não foi possível alcançar o Google Chat")).toBeVisible();
  await page.getByRole("button", { name: "Tentar novamente" }).click();
  await expect(page.getByRole("button", { name: /Bruno Lima/ })).toBeVisible();
  await expect(page.getByText("Não foi possível alcançar o Google Chat")).toHaveCount(0);
});

test("filtro e abas restringem a lista carregada", async ({ page }) => {
  await withBridge(page);
  await page.goto("/");
  await page.getByRole("button", { name: "Espaços" }).click();
  await expect(page.getByRole("button", { name: /Bruno Lima/ })).toHaveCount(0);
  await page.getByRole("button", { name: "Tudo" }).click();
  await page.getByRole("textbox", { name: "Filtrar conversas carregadas" }).fill("zzz");
  await expect(page.getByText("Nenhuma conversa corresponde ao filtro.")).toBeVisible();
});

test("tooltip fica aberto enquanto o mouse está no ícone", async ({ page }) => {
  // O FluentProvider copia a className para o contêiner dos portais. Se ela for
  // a do layout, o contêiner cobre a tela, o ícone perde o hover e o tooltip pisca.
  await page.goto("/");
  const button = page.getByRole("button", { name: "Usar tema escuro" });
  await button.hover();
  await expect(page.getByRole("tooltip")).toBeVisible();
  for (let i = 0; i < 8; i++) {
    await page.waitForTimeout(100);
    await expect(page.getByRole("tooltip")).toHaveCount(1);
  }
  const box = await button.boundingBox();
  if (!box) throw new Error("botão sem posição");
  const onTop = await page.evaluate(
    ([x, y]) => document.elementFromPoint(x, y)?.closest("button")?.getAttribute("aria-label"),
    [box.x + box.width / 2, box.y + box.height / 2],
  );
  expect(onTop).toBe("Usar tema escuro");
});

test.describe("vários projetos Google Cloud", () => {
  const settings = (page: import("@playwright/test").Page) => page.getByRole("dialog", { name: "Configurações" });

  test("com um projeto só, o + conecta direto por ele", async ({ page }) => {
    await withBridge(page, { projects: 1 });
    await page.goto("/");
    await page.getByRole("button", { name: "Adicionar conta Google" }).click();
    await expect
      .poll(() => calls(page, "ConnectAccount"))
      .toEqual([{ method: "ConnectAccount", args: ["123-abc.apps.googleusercontent.com"] }]);
  });

  test("com dois, o + abre as configurações e cada projeto conecta pelo seu", async ({ page }) => {
    await withBridge(page, { projects: 2 });
    await page.goto("/");
    await page.getByRole("button", { name: "Adicionar conta Google" }).click();
    const dialog = settings(page);
    await expect(dialog.getByText("escolha o projeto dela na lista abaixo")).toBeVisible();
    expect(await calls(page, "ConnectAccount")).toHaveLength(0);

    const second = dialog.locator(".settings-project").filter({ hasText: "456-def" });
    await expect(second).toContainText("Projeto 456");
    await second.getByRole("button", { name: "Adicionar conta" }).click();
    await expect
      .poll(() => calls(page, "ConnectAccount"))
      .toEqual([{ method: "ConnectAccount", args: ["456-def.apps.googleusercontent.com"] }]);
  });

  test("conta mostra por qual projeto entrou", async ({ page }) => {
    await withBridge(page, { projects: 2 });
    await page.goto("/");
    await page.getByRole("button", { name: "Configurações e contas" }).click();
    const dialog = settings(page);
    await expect(dialog.getByText("ana@empresa-a.com · chat-empresa-a")).toBeVisible();
    await expect(dialog.getByText("ana@consultoria-b.com.br · Projeto 456")).toBeVisible();
  });

  test("projeto com conta não sai; sem conta, sai", async ({ page }) => {
    await withBridge(page, { projects: 2, accounts: 1 });
    await page.goto("/");
    await page.getByRole("button", { name: "Configurações e contas" }).click();
    const dialog = settings(page);
    const used = dialog.locator(".settings-project").filter({ hasText: "123-abc" });
    const empty = dialog.locator(".settings-project").filter({ hasText: "456-def" });
    await expect(used).toContainText("1 conta");
    await expect(empty).toContainText("Nenhuma conta");

    await used.getByRole("button", { name: "Remover" }).click({ force: true });
    expect(await calls(page, "RemoveProject")).toHaveLength(0);

    await empty.getByRole("button", { name: "Remover" }).click();
    await expect(empty).toHaveCount(0);
    expect((await calls(page, "RemoveProject")).map((c) => c.args[0])).toEqual(["456-def.apps.googleusercontent.com"]);
  });

  test("sem projeto, as configurações dizem o que falta em vez de um botão desativado", async ({ page }) => {
    await withBridge(page, { configured: false });
    await page.goto("/");
    await page.getByRole("button", { name: "Configurações e contas" }).click();
    const dialog = settings(page);
    await expect(dialog.getByText("Importe o arquivo JSON de um projeto")).toBeVisible();
    await expect(dialog.getByRole("button", { name: "Importar arquivo JSON" })).toBeEnabled();
  });
});

test("durante o login dá para copiar o link e abrir em outro container", async ({ page }) => {
  await withBridge(page, { delays: { ConnectAccount: 5000 } });
  await page.goto("/");
  await page.getByRole("button", { name: "Adicionar conta Google" }).click();
  const copy = page.getByRole("button", { name: "Copiar link" });
  await expect(copy).toBeVisible();
  await expect(copy).toBeEnabled();
  await copy.click();
  await expect(page.getByRole("button", { name: "Link copiado" })).toBeVisible();
  expect(await calls(page, "CopyLoginURL")).toHaveLength(1);
  await page.getByRole("button", { name: "Cancelar" }).click();
  expect(await calls(page, "CancelLogin")).toHaveLength(1);
});

test.describe("passo a passo de conexão", () => {
  test("abre pela tela inicial, copia os escopos e leva às páginas do console", async ({ page }) => {
    await withBridge(page, { configured: false });
    await page.goto("/");
    await page.getByRole("button", { name: "Como conectar" }).click();
    const guide = page.getByRole("dialog", { name: "Como conectar" });
    await expect(guide.getByRole("heading", { name: "Com público Externo: teste ou produção" })).toBeVisible();

    await guide.getByRole("button", { name: "Copiar escopos" }).click();
    await expect(guide.getByRole("button", { name: "Escopos copiados" })).toBeVisible();
    const [copy] = await calls(page, "CopyText");
    expect(String(copy.args[0]).split("\n")).toEqual([
      "openid",
      "email",
      "profile",
      "https://www.googleapis.com/auth/chat.spaces.readonly",
      "https://www.googleapis.com/auth/chat.memberships.readonly",
      "https://www.googleapis.com/auth/chat.messages",
    ]);

    await guide.getByRole("button", { name: "Público" }).click();
    await guide.getByRole("button", { name: "Clientes" }).click();
    expect((await calls(page, "OpenLink")).map((c) => c.args[0])).toEqual([
      "https://console.cloud.google.com/auth/audience",
      "https://console.cloud.google.com/auth/clients",
    ]);
    await guide.getByRole("button", { name: "Entendi" }).click();
    await expect(guide).toHaveCount(0);
  });

  test("abre pelas configurações", async ({ page }) => {
    await withBridge(page);
    await page.goto("/");
    await page.getByRole("button", { name: "Configurações e contas" }).click();
    await page.getByRole("button", { name: "Como conectar: passo a passo" }).click();
    await expect(page.getByRole("dialog", { name: "Configurações" })).toHaveCount(0);
    await expect(page.getByRole("dialog", { name: "Como conectar" })).toBeVisible();
  });
});

test("aviso com URL longa quebra linha e o link abre no navegador", async ({ page }) => {
  const url = "https://console.developers.google.com/apis/api/chat.googleapis.com/overview?project=760952425460";
  await withBridge(page, {
    failOnce: {
      Spaces: `a Google Chat API não está ativada. O Google disse: enable it by visiting ${url} then retry.`,
    },
  });
  await page.goto("/");
  const notice = page.locator(".list-notice .fui-MessageBar");
  await expect(notice).toBeVisible();
  const [box, list] = await Promise.all([notice.boundingBox(), page.locator(".conversations").boundingBox()]);
  if (!box || !list) throw new Error("sem posição");
  expect(box.x + box.width).toBeLessThanOrEqual(list.x + list.width);
  expect(await notice.evaluate((e) => e.scrollWidth <= e.clientWidth)).toBe(true);
  await notice.getByRole("link", { name: url }).click();
  await expect.poll(() => calls(page, "OpenLink")).toEqual([{ method: "OpenLink", args: [url] }]);
});

test.describe("notificações e não lidas", () => {
  const unread = [
    { accountId: "111", space: "spaces/dm111", count: 3 },
    { accountId: "222", space: "spaces/team222", count: 2 },
  ];

  test("contadores na faixa e na lista, e abrir a conversa avisa o Go", async ({ page }) => {
    await withBridge(page);
    await page.goto("/");
    await expect(page.getByRole("button", { name: /Bruno Lima/ })).toBeVisible();
    await page.evaluate((u) => window.bridge.emit("unread:changed", u), unread);

    await expect(account(page, "ana@empresa-a.com, 3 não lidas")).toBeVisible();
    await expect(account(page, "ana@consultoria-b.com.br, 2 não lidas")).toBeVisible();
    await expect(page.getByRole("button", { name: "Bruno Lima Conversa direta 3 não lidas" })).toBeVisible();

    await page.getByRole("button", { name: /Bruno Lima/ }).click();
    await expect
      .poll(async () => (await calls(page, "SetOpenConversation")).at(-1)?.args)
      .toEqual(["111", "spaces/dm111"]);
    await expect(page.getByRole("button", { name: /Bruno Lima/ }).locator(".unread-badge")).toHaveCount(0);
  });

  test("clicar na notificação troca de conta e abre a conversa, mesmo fora da lista", async ({ page }) => {
    await withBridge(page);
    await page.goto("/");
    await expect(page.getByRole("button", { name: /Bruno Lima/ })).toBeVisible();
    const space = {
      name: "spaces/nova222",
      displayName: "Carla Dias",
      spaceType: "DIRECT_MESSAGE",
      spaceThreadingState: "UNTHREADED_MESSAGES",
      lastActiveTime: new Date().toISOString(),
      spaceUri: "",
    };
    await page.evaluate(
      (s) => window.bridge.emit("open:conversation", { accountId: "222", space: s, message: {} }),
      space,
    );

    await expect(account(page, "ana@consultoria-b.com.br")).toHaveAttribute("aria-pressed", "true");
    await expect(page.getByRole("heading", { name: "Carla Dias" })).toBeVisible();
    await expect
      .poll(async () => (await calls(page, "Messages")).at(-1)?.args.slice(0, 2))
      .toEqual(["222", "spaces/nova222"]);
  });

  test("configurações ligam, desligam e mudam o intervalo", async ({ page }) => {
    await withBridge(page);
    await page.goto("/");
    await page.getByRole("button", { name: "Configurações e contas" }).click();
    const dialog = page.getByRole("dialog", { name: "Configurações" });
    const every = dialog.getByLabel("Verificar a cada");
    await expect(every).toHaveValue("60");

    await every.selectOption("15");
    await expect
      .poll(async () => (await calls(page, "SetNotifications")).at(-1)?.args[0])
      .toEqual({ on: true, every: 15 });

    await dialog.getByRole("switch", { name: "Avisar mensagens novas" }).click();
    await expect
      .poll(async () => (await calls(page, "SetNotifications")).at(-1)?.args[0])
      .toEqual({ on: false, every: 15 });
    await expect(every).toBeDisabled();
  });
});

test.describe("arquivos", () => {
  const openDM = async (page: import("@playwright/test").Page) => {
    await page.goto("/");
    await page.getByRole("button", { name: /Bruno Lima/ }).click();
    return page.getByRole("textbox", { name: "Escrever mensagem" });
  };
  const sends = async (page: import("@playwright/test").Page) =>
    (await calls(page, "SendMessage")).map((c) => ({ text: c.args[2], requestId: c.args[4], ref: c.args[5] }));

  test("dois arquivos com texto viram duas mensagens, o texto na primeira", async ({ page }) => {
    await withBridge(page);
    const box = await openDM(page);
    await box.fill("segue o material");
    await page.getByRole("button", { name: "Anexar arquivos" }).click();
    const list = page.getByRole("list", { name: "Arquivos para enviar" });
    await expect(list.getByRole("listitem")).toHaveCount(2);
    await expect(list).toContainText("relatorio.pdf");
    await expect(list).toContainText("2,4 MB");
    await box.press("Enter");

    await expect(list).toHaveCount(0);
    await expect(box).toHaveValue("");
    const sent = await sends(page);
    expect(sent.map((m) => [m.text, (m.ref as { attachmentUploadToken: string }).attachmentUploadToken])).toEqual([
      ["segue o material", "tok-f1"],
      ["", "tok-f2"],
    ]);
    expect(sent[0].requestId).not.toBe(sent[1].requestId);
    expect((await calls(page, "ForgetFile")).map((c) => c.args[0])).toEqual(["f1", "f2"]);
  });

  test("falha no meio continua de onde parou, sem duplicar", async ({ page }) => {
    // O primeiro arquivo sobe e vira mensagem; o segundo falha no upload.
    await withBridge(page, { failOnce: { "UploadAttachment:f2": "o upload foi interrompido" } });
    const box = await openDM(page);
    await box.fill("segue o material");
    await page.getByRole("button", { name: "Anexar arquivos" }).click();
    await box.press("Enter");
    await expect(page.getByText("O upload foi interrompido")).toBeVisible();
    await expect(page.getByText("O que faltou enviar ficou aqui")).toBeVisible();
    const list = page.getByRole("list", { name: "Arquivos para enviar" });
    await expect(list.getByRole("listitem")).toHaveCount(1);
    await expect(list).toContainText("foto.png");
    await expect(box).toHaveValue("");

    await box.press("Enter");
    await expect(list).toHaveCount(0);
    expect((await calls(page, "UploadAttachment")).map((c) => c.args[2])).toEqual(["f1", "f2", "f2"]);
    expect((await sends(page)).map((m) => m.text)).toEqual(["segue o material", ""]);
  });

  test("falha no envio depois do upload reaproveita o arquivo e o requestId", async ({ page }) => {
    await withBridge(page, { failOnce: { "SendMessage:111": "sem conexão" } });
    const box = await openDM(page);
    await page.getByRole("button", { name: "Anexar arquivos" }).click();
    await page.getByRole("button", { name: "Tirar foto.png" }).click();
    await box.press("Enter");
    await expect(page.getByText("Sem conexão")).toBeVisible();
    await box.press("Enter");
    await expect(page.getByRole("list", { name: "Arquivos para enviar" })).toHaveCount(0);

    expect(await calls(page, "UploadAttachment")).toHaveLength(1);
    const [first, retry] = await sends(page);
    expect(retry.requestId).toBe(first.requestId);
    expect(retry.ref).toEqual(first.ref);
  });

  test("arquivo acima de 200 MB trava o envio até sair da lista", async ({ page }) => {
    await withBridge(page, { pickFiles: [{ id: "g", name: "video.mov", size: 300 * 1024 * 1024, tooLarge: true }] });
    const box = await openDM(page);
    await box.fill("olha o vídeo");
    await page.getByRole("button", { name: "Anexar arquivos" }).click();
    await expect(page.getByText("mais de 200 MB, o limite")).toBeVisible();
    const send = page.getByRole("button", { name: "Enviar mensagem" });
    await expect(send).toBeDisabled();
    await page.getByRole("button", { name: "Tirar video.mov" }).click();
    await expect(send).toBeEnabled();
    expect((await calls(page, "ForgetFile")).map((c) => c.args[0])).toEqual(["g"]);
  });

  test("anexo recebido: baixar, mostrar na pasta e abrir arquivo do Drive", async ({ page }) => {
    await withBridge(page);
    await openDM(page);
    await expect(page.getByText("Conteúdo disponível no Google Chat")).toHaveCount(0);
    await page.getByRole("button", { name: "Baixar planilha.xlsx" }).click();
    await expect
      .poll(async () => (await calls(page, "DownloadAttachment"))[0]?.args)
      .toEqual(["111", "recurso/planilha", "planilha.xlsx"]);
    await page.getByRole("button", { name: "Salvo · mostrar na pasta" }).click();
    await expect
      .poll(async () => (await calls(page, "ShowInFolder"))[0]?.args)
      .toEqual(["/home/teste/Downloads/planilha.xlsx"]);

    await page.getByRole("button", { name: "Abrir no Drive" }).click();
    await expect
      .poll(async () => (await calls(page, "OpenLink")).at(-1)?.args)
      .toEqual(["https://drive.google.com/file/d/drive123/view"]);
  });
});

test.describe("idioma", () => {
  test.describe("navegador em inglês", () => {
    test.use({ locale: "en-US" });
    test("o app segue o idioma do sistema", async ({ page }) => {
      await withBridge(page);
      await page.goto("/");
      await expect(page.getByRole("button", { name: "Add Google account" })).toBeVisible();
      await expect(page.getByPlaceholder("Filter conversations")).toBeVisible();
      await expect(page.locator("html")).toHaveAttribute("lang", "en-US");
    });
  });

  test("trocar nas configurações muda a tela na hora e avisa o Go", async ({ page }) => {
    await withBridge(page);
    await page.goto("/");
    await page.getByRole("button", { name: "Configurações e contas" }).click();
    await page.getByLabel("Idioma").selectOption("en");

    const dialog = page.getByRole("dialog", { name: "Settings" });
    await expect(dialog.getByRole("heading", { name: "Notifications" })).toBeVisible();
    await expect(dialog.getByLabel("Check every").locator("option[value='60']")).toHaveText("1 minute");
    await expect.poll(async () => (await calls(page, "SetLanguage")).map((c) => c.args[0])).toEqual(["en"]);
    await expect(page.locator("html")).toHaveAttribute("lang", "en");

    await dialog.getByLabel("Language").selectOption("");
    await expect(page.getByRole("dialog", { name: "Configurações" })).toBeVisible();
    await expect.poll(async () => (await calls(page, "SetLanguage")).map((c) => c.args[0])).toEqual(["en", ""]);
  });

  test("datas e tamanhos seguem o idioma", async ({ page }) => {
    await withBridge(page);
    await page.goto("/");
    await page.getByRole("button", { name: /Bruno Lima/ }).click();
    await expect(page.locator(".date-divider").first()).toContainText("de outubro de 2026");
    await page.getByRole("button", { name: "Configurações e contas" }).click();
    await page.getByLabel("Idioma").selectOption("en");
    await page.getByRole("button", { name: "Done" }).click();
    await expect(page.locator(".date-divider").first()).toContainText("October");
    await page.getByRole("button", { name: "Attach files" }).click();
    await expect(page.getByRole("list", { name: "Files to send" })).toContainText("2.4 MB");
  });
});
