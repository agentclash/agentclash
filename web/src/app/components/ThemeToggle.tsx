"use client";

import { useState, useEffect } from "react";

export default function ThemeToggle() {
  const [dark, setDark] = useState(true);

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", dark ? "dark" : "light");
  }, [dark]);

  return (
    <button
      onClick={() => setDark(!dark)}
      aria-label="Toggle theme"
      style={{
        position: "fixed",
        bottom: "24px",
        right: "24px",
        zIndex: 9999,
        width: "44px",
        height: "44px",
        borderRadius: "50%",
        border: "1px solid rgba(128,128,128,0.3)",
        background: dark ? "rgba(255,255,255,0.08)" : "rgba(0,0,0,0.06)",
        color: dark ? "#fff" : "#000",
        fontSize: "18px",
        cursor: "pointer",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        backdropFilter: "blur(12px)",
        transition: "all 0.3s ease",
      }}
    >
      {dark ? "☀" : "●"}
    </button>
  );
}
