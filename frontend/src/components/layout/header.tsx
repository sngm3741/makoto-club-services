'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useCallback, useState } from 'react';

import { NAV_LINKS, SITE_NAME } from '@/config/site';

const OFFICIAL_LINE_URL = 'https://lin.ee/KN2j1G5';

export const Header = () => {
  const pathname = usePathname();
  const [isMenuOpen, setIsMenuOpen] = useState(false);

  const handleLineLogin = useCallback(() => {
    setIsMenuOpen(false);

    if (typeof window === 'undefined') return;

    window.location.href = OFFICIAL_LINE_URL;
  }, []);

  return (
    <header className="relative sticky top-0 z-40 w-full border-b border-slate-100 bg-white/90 backdrop-blur">
      <div className="mx-auto flex h-16 max-w-5xl items-center justify-between px-4">
        <div className="flex items-center gap-6">
          <Link href="/" className="font-semibold text-pink-600">
            <span className="text-lg">{SITE_NAME}</span>
          </Link>
          <nav className="hidden gap-4 text-sm font-medium text-slate-700 md:flex">
            {NAV_LINKS.map((link) => {
              const isActive =
                pathname === link.href || pathname.startsWith(`${link.href}/`);
              return (
                <Link
                  key={link.href}
                  href={link.href}
                  onClick={() => setIsMenuOpen(false)}
                  className={`rounded-full px-3 py-1 transition ${
                    isActive ? 'bg-pink-100 text-pink-700' : 'hover:bg-slate-100'
                  }`}
                >
                  {link.label}
                </Link>
              );
            })}
          </nav>
        </div>

        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={handleLineLogin}
            className="hidden rounded-full bg-gradient-to-r from-pink-500 to-violet-500 px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:from-pink-400 hover:to-violet-400 md:inline-block"
          >
            相談する
          </button>
          <button
            type="button"
            className="inline-flex h-10 w-10 items-center justify-center rounded-full bg-slate-100 text-slate-700 transition hover:bg-slate-200 md:hidden"
            onClick={() => setIsMenuOpen((prev) => !prev)}
            aria-label="メニューを開く"
            aria-expanded={isMenuOpen}
          >
            <span className="relative block h-5 w-6">
              <span
                className={`absolute left-0 h-0.5 w-full rounded bg-current transition ${
                  isMenuOpen ? 'top-2.5 rotate-45' : 'top-1'
                }`}
              />
              <span
                className={`absolute left-0 h-0.5 w-full rounded bg-current transition ${
                  isMenuOpen ? 'opacity-0' : 'top-2.5'
                }`}
              />
              <span
                className={`absolute left-0 h-0.5 w-full rounded bg-current transition ${
                  isMenuOpen ? 'top-2.5 -rotate-45' : 'top-4'
                }`}
              />
            </span>
          </button>
        </div>
      </div>

      <div
        className={`md:hidden ${isMenuOpen ? 'pointer-events-auto opacity-100' : 'pointer-events-none opacity-0'} absolute inset-x-0 top-16 bg-white shadow-lg transition`}
      >
        <nav className="flex flex-col gap-2 px-4 py-4 text-sm font-medium text-slate-700">
          {NAV_LINKS.map((link) => {
            const isActive =
              pathname === link.href || pathname.startsWith(`${link.href}/`);
            return (
              <Link
                key={link.href}
                href={link.href}
                onClick={() => setIsMenuOpen(false)}
                className={`rounded-lg px-3 py-2 ${
                  isActive ? 'bg-pink-100 text-pink-700' : 'hover:bg-slate-100'
                }`}
              >
                {link.label}
              </Link>
            );
          })}
          <button
            type="button"
            onClick={handleLineLogin}
            className="rounded-lg bg-gradient-to-r from-pink-500 to-violet-500 px-3 py-2 text-center text-white"
          >
            相談する
          </button>
        </nav>
      </div>
    </header>
  );
};
