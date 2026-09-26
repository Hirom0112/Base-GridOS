import { createRootRoute, HeadContent, Scripts } from "@tanstack/react-router";
import stylesheet from "../styles.css?url";

export const Route = createRootRoute({
  head: () => ({
    meta: [
      { charSet: "utf-8" },
      { name: "viewport", content: "width=device-width, initial-scale=1" },
      { title: "GridOS · Operator console" },
    ],
    links: [{ rel: "stylesheet", href: stylesheet }],
  }),
  component: RootDocument,
});

export function RootDocument() {
  return (
    <html lang="en">
      <head>
        <HeadContent />
      </head>
      <body>
        <main>
          <h1>GridOS</h1>
          <p>Operator console</p>
        </main>
        <Scripts />
      </body>
    </html>
  );
}
