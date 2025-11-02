'use client';

type TwitterUser = {
  userId: string;
  username: string;
  displayName: string;
  avatarUrl?: string;
};

export type TwitterLoginResult = {
  accessToken: string;
  twitterUser: TwitterUser;
};

export const AUTH_STORAGE_KEY = 'makotoClubTwitterAuth';
export const AUTH_UPDATE_EVENT = 'twitter-auth:updated';

export type LoginResponseMessage =
  | {
      type: 'oauth-login-result';
      success: true;
      state: string;
      payload: {
        accessToken: string;
        tokenType: string;
        expiresIn: number;
        twitterUser: TwitterUser;
      };
    }
  | {
      type: 'oauth-login-result';
      success: false;
      state?: string;
      error?: string;
    };

function normaliseBaseUrl(baseUrl: string): string {
  return baseUrl.replace(/\/+$/, '');
}

export function readStoredAuth(): TwitterLoginResult | undefined {
  if (typeof window === 'undefined') return undefined;
  const raw = sessionStorage.getItem(AUTH_STORAGE_KEY);
  if (!raw) return undefined;
  try {
    return JSON.parse(raw) as TwitterLoginResult;
  } catch {
    return undefined;
  }
}

export async function startTwitterLogin(baseUrl: string): Promise<void> {
  if (typeof window === 'undefined') {
    throw new Error('クライアント環境でのみ実行できます。');
  }

  if (!baseUrl) {
    throw new Error('Xログインのエンドポイントが設定されていません。');
  }

  const endpoint = normaliseBaseUrl(baseUrl);
  const origin = window.location.origin;

  const response = await fetch(`${endpoint}/twitter/login`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ origin }),
  });

  if (!response.ok) {
    const message = await safeParseError(response);
    throw new Error(message ?? 'Xログインの開始に失敗しました。');
  }

  const { authorizationUrl } = (await response.json()) as {
    authorizationUrl: string;
  };

  if (!authorizationUrl) {
    throw new Error('Xログインの初期化に失敗しました。');
  }

  window.location.assign(authorizationUrl);
}

export function persistAuthResult(result: TwitterLoginResult) {
  if (typeof window === 'undefined') return;
  sessionStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify(result));
  window.dispatchEvent(new CustomEvent<TwitterLoginResult>(AUTH_UPDATE_EVENT, { detail: result }));
}

async function safeParseError(response: Response): Promise<string | undefined> {
  try {
    const data = await response.json();
    if (typeof data?.error === 'string') {
      return data.error;
    }
  } catch {
    // ignore
  }
  return undefined;
}
