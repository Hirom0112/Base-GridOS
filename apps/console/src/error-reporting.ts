import {
  BrowserClient,
  Scope,
  defaultStackParser,
  makeFetchTransport,
} from "@sentry/browser";
import { sentryPayload } from "./observability";

export function startErrorReporting(dsn: string, stage: string) {
  const client = new BrowserClient({
    dsn,
    integrations: [],
    stackParser: defaultStackParser,
    dataCollection: {
      userInfo: false,
      cookies: false,
      httpHeaders: false,
      httpBodies: [],
      urlQueryParams: false,
      stackFrameVariables: false,
      frameContextLines: 0,
    },
    sendClientReports: false,
    transport: (options) =>
      makeFetchTransport(options, (url, init) =>
        fetch(url, {
          ...init,
          credentials: "omit",
          referrerPolicy: "no-referrer",
          signal: AbortSignal.timeout(5000),
        }),
      ),
    beforeSend: (event) => ({
      ...sentryPayload(event, stage),
      type: undefined,
    }),
  });
  const scope = new Scope();
  scope.setClient(client);
  client.init();
  const report = () => scope.captureMessage("Console runtime error", "error");
  window.addEventListener("error", report);
  window.addEventListener("unhandledrejection", report);
  return () => {
    window.removeEventListener("error", report);
    window.removeEventListener("unhandledrejection", report);
    void client.close(1000);
  };
}
