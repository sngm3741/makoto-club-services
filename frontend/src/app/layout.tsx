import type { Metadata } from 'next';
import { Geist, Geist_Mono } from 'next/font/google';

import { Header } from '@/components/layout/header';
import { Footer } from '@/components/layout/footer';
import { TwitterLoginHandler } from '@/components/twitter/twitter-login-handler';
import { SITE_NAME } from '@/config/site';

import './globals.css';

const geistSans = Geist({
  variable: '--font-geist-sans',
  subsets: ['latin'],
});

const geistMono = Geist_Mono({
  variable: '--font-geist-mono',
  subsets: ['latin'],
});

export const metadata: Metadata = {
  title: {
    default: `${SITE_NAME} | 風俗口コミで働き先さがし`,
    template: `%s | ${SITE_NAME}`,
  },
  description:
    '全国の風俗で働く女の子のための口コミ検索サービス。実際のアンケート情報から安心して働ける店舗を探せます。',
  metadataBase:
    process.env.NEXT_PUBLIC_SITE_URL || process.env.SITE_URL
      ? new URL((process.env.NEXT_PUBLIC_SITE_URL || process.env.SITE_URL) as string)
      : undefined,
  openGraph: {
    title: `${SITE_NAME} | 風俗口コミで働き先さがし`,
    description:
      '全国の風俗で働く女の子のための口コミ検索サービス。実際のアンケート情報から安心して働ける店舗を探せます。',
    type: 'website',
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="ja">
      <body
        className={`${geistSans.variable} ${geistMono.variable} min-h-screen bg-slate-50 text-slate-900 antialiased`}
      >
        <TwitterLoginHandler />
        <div className="flex min-h-screen flex-col">
          <Header />
          <main className="flex-1">
            <div className="mx-auto w-full max-w-5xl px-4 py-8">{children}</div>
          </main>
          <Footer />
        </div>
      </body>
    </html>
  );
}
