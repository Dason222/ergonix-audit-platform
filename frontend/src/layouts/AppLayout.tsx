import { NavLink, Outlet, useLocation } from "react-router-dom";

const NAV = [
  { to: "/", label: "Dashboard", icon: "M3 13h4v8H3zM10 7h4v14h-4zM17 3h4v18h-4z" },
  { to: "/audits", label: "Audit History", icon: "M4 5h16M4 12h16M4 19h10" },
  { to: "/audits/new", label: "New Audit", icon: "M12 4v16M4 12h16" },
  { to: "/settings", label: "Settings", icon: "M12 8a4 4 0 100 8 4 4 0 000-8zM19 12h2M3 12h2M12 3v2M12 19v2M17.7 6.3l1.4-1.4M4.9 19.1l1.4-1.4M17.7 17.7l1.4 1.4M4.9 4.9l1.4 1.4" },
];

const TITLES: Record<string, string> = {
  "/": "Operations Dashboard",
  "/audits": "Audit History",
  "/audits/new": "Create Audit",
  "/settings": "Settings",
};

export default function AppLayout() {
  const { pathname } = useLocation();
  const title =
    TITLES[pathname] ??
    (pathname.startsWith("/audits/") ? "Audit Report" : "Ergonix Audit");

  return (
    <div className="flex min-h-screen">
      <aside className="blueprint fixed inset-y-0 left-0 z-20 flex w-56 flex-col bg-ink-900 text-slate-300">
        <div className="flex items-center gap-2.5 border-b border-white/10 px-5 py-4">
          <svg width="26" height="26" viewBox="0 0 32 32">
            <rect width="32" height="32" rx="6" fill="#1a2537" />
            <path
              d="M8 22 L14 10 L18 18 L21 13 L24 22"
              stroke="#2dd4bf"
              strokeWidth="2.5"
              fill="none"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
          <div>
            <div className="text-[13px] font-semibold leading-tight text-white">
              Ergonix Audit
            </div>
            <div className="font-mono text-[10px] uppercase tracking-widest text-signal-300/70">
              console
            </div>
          </div>
        </div>

        <nav className="flex-1 space-y-0.5 px-3 py-4">
          {NAV.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === "/" || item.to === "/audits"}
              className={({ isActive }) =>
                `flex items-center gap-2.5 rounded-md px-3 py-2 text-[13px] transition-colors ${
                  isActive
                    ? "bg-white/10 font-medium text-white shadow-[inset_2px_0_0_0_#2dd4bf]"
                    : "text-slate-400 hover:bg-white/5 hover:text-slate-200"
                }`
              }
            >
              <svg
                width="15"
                height="15"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.8"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <path d={item.icon} />
              </svg>
              {item.label}
            </NavLink>
          ))}
        </nav>

        <div className="border-t border-white/10 px-5 py-3">
          <div className="font-mono text-[10px] leading-relaxed text-slate-500">
            ERGONIX QA PLATFORM
            <br />
            v1.0 · internal tool
          </div>
        </div>
      </aside>

      <div className="ml-56 flex-1">
        <header className="sticky top-0 z-10 flex h-13 items-center justify-between border-b border-line bg-panel/90 px-6 py-3 backdrop-blur">
          <h1 className="text-[15px] font-semibold tracking-tight">{title}</h1>
          <div className="font-mono text-[11px] text-ink-400">
            {new Date().toLocaleDateString(undefined, {
              weekday: "short",
              year: "numeric",
              month: "short",
              day: "numeric",
            })}
          </div>
        </header>
        <main className="mx-auto max-w-7xl px-6 py-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
