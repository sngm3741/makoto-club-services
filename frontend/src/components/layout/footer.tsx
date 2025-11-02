import Link from 'next/link';

import { FOOTER_LINKS, SITE_NAME } from '@/config/site';

export const Footer = () => {
  const currentYear = new Date().getFullYear();

  return (
    <footer className="border-t border-slate-100 bg-white py-10">
      <div className="mx-auto flex max-w-5xl flex-col gap-6 px-4 text-sm text-slate-600 md:flex-row md:items-center md:justify-between">
        <div>
          <p className="font-semibold text-slate-800">{SITE_NAME}</p>
          <p className="mt-1 text-xs text-slate-500">
            みんなのリアルな声で、安心して働けるお店選びを。
          </p>
        </div>
        <nav className="flex flex-wrap gap-x-6 gap-y-2">
          {FOOTER_LINKS.map((link) => (
            <Link key={link.href} href={link.href} className="hover:text-pink-600">
              {link.label}
            </Link>
          ))}
        </nav>
      </div>
      <p className="mt-6 text-center text-xs text-slate-400">
        © {currentYear} {SITE_NAME}. All rights reserved.
      </p>
    </footer>
  );
};
