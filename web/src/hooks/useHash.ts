import { useState, useCallback, useEffect } from "react";

function readHash(): string {
  return window.location.hash.replace(/^#/, "") || "/";
}

export function useHash() {
  const [hash, setHash] = useState(readHash);

  useEffect(() => {
    const onHashChange = () => setHash(readHash());
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  }, []);

  const navigate = useCallback((path: string) => {
    window.location.hash = path.startsWith("#") ? path : `#${path}`;
  }, []);

  return { hash, navigate };
}
