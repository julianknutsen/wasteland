import "./api/prefetch"; // Start browse fetch immediately, before React mounts.
import { getDefaultIntegrations, globalHandlersIntegration } from "@sentry/browser";
import * as Sentry from "@sentry/react";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import { Toaster } from "./components/Toaster";
import "./styles/global.css";

Sentry.init({
  dsn: import.meta.env.VITE_SENTRY_DSN || "",
  environment: import.meta.env.VITE_ENVIRONMENT || "development",
  integrations: [
    ...getDefaultIntegrations({}),
    globalHandlersIntegration({ onerror: true, onunhandledrejection: true }),
    Sentry.browserTracingIntegration(),
    Sentry.replayIntegration(),
  ],
  tracesSampleRate: 0.2,
  replaysSessionSampleRate: 0,
  replaysOnErrorSampleRate: 1.0,
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
    <Toaster />
  </StrictMode>,
);
