import { useEffect, useState, useMemo } from "react";
import { useHash } from "./hooks/useHash";
import { NavPillGroup } from "./components/NavPillGroup";
import { StatsDashboard } from "./pages/StatsDashboard";
import { ProviderManager } from "./pages/ProviderManager";
import { LinkManager } from "./pages/LinkManager";
import { LinkEditor } from "./pages/LinkEditor";
import { api } from "./api";
import { Spinner } from "./components/Spinner";
import type { ProxyLink } from "./types";

const tabs = [
  { key: "/stats", label: "Statistics" },
  { key: "/providers", label: "Providers" },
  { key: "/links", label: "Links" },
];

function App() {
  const { hash, navigate } = useHash();

  useEffect(() => {
    if (!window.location.hash || window.location.hash === "#") {
      navigate("/stats");
    }
  }, [navigate]);

  const activeTab = hash.startsWith("/") ? hash : `/${hash}`;

  const isActive = (path: string) => activeTab === path || activeTab.startsWith(path + "/");

  const [editingLink, setEditingLink] = useState<ProxyLink | null>(null);
  const [loadingLink, setLoadingLink] = useState(false);
  const editId = useMemo(() => {
    if (activeTab.startsWith("/links/edit/")) {
      return activeTab.replace("/links/edit/", "");
    }
    return null;
  }, [activeTab]);

  useEffect(() => {
    if (editId) {
      setLoadingLink(true);
      api.getLink(editId)
        .then(setEditingLink)
        .catch(() => setEditingLink(null))
        .finally(() => setLoadingLink(false));
    } else {
      setEditingLink(null);
    }
  }, [editId]);

  const onLinkSaved = () => {
    navigate("/links");
  };

  const renderContent = () => {
    if (activeTab === "/links/new") {
      return <LinkEditor onSaved={onLinkSaved} onCancel={() => navigate("/links")} />;
    }
    if (editId) {
      if (loadingLink) return <Spinner />;
      return <LinkEditor link={editingLink} onSaved={onLinkSaved} onCancel={() => navigate("/links")} />;
    }
    if (isActive("/stats")) return <StatsDashboard />;
    if (isActive("/providers")) return <ProviderManager />;
    if (isActive("/links")) return <LinkManager />;
    return null;
  };

  return (
    <div className="min-h-screen flex flex-col bg-[var(--color-canvas)]">
      <header className="sticky top-0 z-40 h-16 bg-[var(--color-canvas)] border-b border-[var(--color-hairline-soft)]">
        <div className="mx-auto flex h-full max-w-[1200px] items-center justify-between px-6">
          <span className="text-lg font-semibold tracking-tight text-[var(--color-ink)]">
            LLM Gateway
          </span>
          <NavPillGroup items={tabs} active={isActive("/links") ? "/links" : activeTab} onChange={navigate} />
        </div>
      </header>

      <main className="flex-1 mx-auto w-full max-w-[1200px] px-6 py-section">
        {renderContent()}
      </main>

      <footer className="bg-[var(--color-surface-dark)] text-[var(--color-on-dark-soft)]">
        <div className="mx-auto max-w-[1200px] px-6 py-16">
          <div className="flex flex-col sm:flex-row items-center justify-between gap-6">
            <span className="text-sm font-semibold text-[var(--color-on-dark)] tracking-tight">
              LLM Gateway
            </span>
            <div className="flex gap-8 text-sm">
              <span>Built by Jachy</span>
              <span>v0.1.0</span>
            </div>
          </div>
        </div>
      </footer>
    </div>
  );
}

export default App;
