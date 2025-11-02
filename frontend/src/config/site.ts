export const SITE_NAME = 'マコトクラブ';

export const NAV_LINKS = [
  { href: '/stores', label: '店舗一覧' },
  { href: '/reviews', label: 'アンケート一覧' },
  { href: '/reviews/new', label: 'アンケート投稿' },
] as const;

export const FOOTER_LINKS = [
  { href: '/terms', label: '利用規約' },
  { href: '/privacy', label: 'プライバシーポリシー' },
  { href: '/contact', label: 'お問い合わせ（削除依頼）' },
] as const;

export const DEFAULT_LINE_LOGIN_PATH = '/line/login';
