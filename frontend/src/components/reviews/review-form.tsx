'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';

import {
  AGE_OPTIONS,
  AVERAGE_EARNING_OPTIONS,
  PREFECTURES,
  REVIEW_CATEGORIES,
  SPEC_MAX,
  SPEC_MAX_LABEL,
  SPEC_MIN,
  SPEC_MIN_LABEL,
  WAIT_TIME_OPTIONS,
} from '@/constants/filters';
import {
  AUTH_UPDATE_EVENT,
  TwitterLoginResult,
  readStoredAuth,
  startTwitterLogin,
} from '@/lib/twitter-auth';

type FormValues = {
  storeName: string;
  branchName: string;
  prefecture: string;
  category: string;
  visitedAt: string;
  age: number;
  specScore: number;
  waitTimeHours: number;
  averageEarning: number;
  comment: string;
  rating: number;
  contactEmail: string;
};

const DEFAULT_FORM_VALUES: FormValues = {
  storeName: '',
  branchName: '',
  prefecture: '',
  category: '',
  visitedAt: '',
  age: 20,
  specScore: 100,
  waitTimeHours: 12,
  averageEarning: 0,
  comment: '',
  rating: 3,
  contactEmail: '',
};

const TWITTER_AUTH_BASE_URL = process.env.NEXT_PUBLIC_TWITTER_AUTH_BASE_URL ?? '';
const PENDING_REVIEW_STORAGE_KEY = 'makotoClubPendingReview';

const storePendingReview = (values: FormValues) => {
  if (typeof window === 'undefined') return;
  try {
    sessionStorage.setItem(PENDING_REVIEW_STORAGE_KEY, JSON.stringify(values));
  } catch (error) {
    console.error('Pending review の保存に失敗しました', error);
  }
};

const readPendingReview = (): FormValues | undefined => {
  if (typeof window === 'undefined') return undefined;
  const raw = sessionStorage.getItem(PENDING_REVIEW_STORAGE_KEY);
  if (!raw) return undefined;
  try {
    const parsed = JSON.parse(raw) as Partial<FormValues>;
    return { ...DEFAULT_FORM_VALUES, ...parsed };
  } catch {
    return undefined;
  }
};

const clearPendingReview = () => {
  if (typeof window === 'undefined') return;
  sessionStorage.removeItem(PENDING_REVIEW_STORAGE_KEY);
};

const RATING_MIN = 0;
const RATING_MAX = 5;
const RATING_STEP = 0.5;

const formatSpecScoreLabel = (value: number) => {
  if (value <= SPEC_MIN) {
    return SPEC_MIN_LABEL;
  }
  if (value >= SPEC_MAX) {
    return SPEC_MAX_LABEL;
  }
  return `${value}`;
};

const formatAverageEarningLabel = (value: number) =>
  value === 20 ? '20万円以上' : `${value}万円`;

const StarDisplay = ({ value }: { value: number }) => {
  const clamped = Math.max(RATING_MIN, Math.min(RATING_MAX, value));
  return (
    <span className="relative inline-block text-xl leading-none">
      <span className="text-slate-300">★★★★★</span>
      <span
        className="absolute left-0 top-0 overflow-hidden text-yellow-400"
        style={{ width: `${(clamped / RATING_MAX) * 100}%` }}
      >
        ★★★★★
      </span>
    </span>
  );
};

export const ReviewForm = () => {
  const [auth, setAuth] = useState<TwitterLoginResult | undefined>();
  const [status, setStatus] = useState<'idle' | 'submitting' | 'success' | 'error'>('idle');
  const [errorMessage, setErrorMessage] = useState('');
  const [authLoading, setAuthLoading] = useState(false);
  const [showSuccessModal, setShowSuccessModal] = useState(false);
  const hasAutoSubmitted = useRef(false);

  const {
    register,
    control,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<FormValues>({
    defaultValues: DEFAULT_FORM_VALUES,
  });

  useEffect(() => {
    if (typeof window === 'undefined') return;
    const current = readStoredAuth();
    if (current) {
      setAuth(current);
    }

    const listener: EventListener = (event) => {
      const custom = event as CustomEvent<TwitterLoginResult>;
      if (!custom.detail) return;
      setAuth(custom.detail);
      setErrorMessage('');
      setStatus('idle');
      hasAutoSubmitted.current = false;
    };

    window.addEventListener(AUTH_UPDATE_EVENT, listener);
    return () => {
      window.removeEventListener(AUTH_UPDATE_EVENT, listener);
    };
  }, []);

  const handleTwitterLogin = useCallback(async () => {
    if (typeof window === 'undefined') return;
    if (!TWITTER_AUTH_BASE_URL) {
      setErrorMessage('Xログインのエンドポイントが設定されていません。');
      setStatus('error');
      return;
    }

    setAuthLoading(true);
    setErrorMessage('');

    try {
      await startTwitterLogin(TWITTER_AUTH_BASE_URL);
      setStatus('idle');
    } catch (error) {
      console.error(error);
      setErrorMessage(
        error instanceof Error
          ? error.message
          : 'Xログインに失敗しました。時間を置いて再度お試しください。',
      );
      setStatus('error');
    } finally {
      setAuthLoading(false);
    }
  }, []);

  const onSubmit = useCallback(
    async (values: FormValues) => {
      if (!auth?.accessToken) {
        storePendingReview(values);
        hasAutoSubmitted.current = false;
        handleTwitterLogin();
        return;
      }

      setStatus('submitting');
      setErrorMessage('');

      const payload = {
        storeName: values.storeName.trim(),
        branchName: values.branchName.trim(),
        prefecture: values.prefecture,
        category: values.category,
        visitedAt: values.visitedAt,
        age: values.age,
        specScore: values.specScore,
        waitTimeHours: values.waitTimeHours,
        averageEarning: values.averageEarning,
        comment: values.comment.trim(),
        rating: values.rating,
        contactEmail: values.contactEmail.trim(),
      };

      const apiBase = process.env.NEXT_PUBLIC_API_BASE_URL ?? '';

      try {
        if (!apiBase) {
          // API 確定前の仮実装: 成功したふりをしてプレビューできるようにする
          console.info('投稿ペイロード', payload);
          await new Promise((resolve) => setTimeout(resolve, 800));
        } else {
          const response = await fetch(`${apiBase}/api/reviews`, {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
              Authorization: `Bearer ${auth.accessToken}`,
            },
            body: JSON.stringify(payload),
          });

          const data = await response.json().catch(() => null);
          if (!response.ok) {
            if (response.status === 401) {
              storePendingReview(values);
              setStatus('idle');
              setErrorMessage('');
              hasAutoSubmitted.current = false;
              setAuth(undefined);
              handleTwitterLogin();
              return;
            }
            const message =
              data &&
              typeof data === 'object' &&
              data !== null &&
              'error' in data &&
              typeof data.error === 'string'
                ? data.error
                : '投稿に失敗しました。時間を置いて再度お試しください。';
            throw new Error(message);
          }

          if (data && typeof window !== 'undefined') {
            console.info('投稿結果', data);
          }
        }

        setStatus('success');
        reset(DEFAULT_FORM_VALUES);
        clearPendingReview();
        hasAutoSubmitted.current = false;
        setShowSuccessModal(true);
      } catch (error) {
        console.error(error);
        setErrorMessage('投稿に失敗しました。時間を置いて再度お試しください。');
        setStatus('error');
      }
    },
    [auth, handleTwitterLogin, reset],
  );

  useEffect(() => {
    if (typeof window === 'undefined') return;
    if (!auth?.accessToken) return;
    if (hasAutoSubmitted.current) return;

    const pending = readPendingReview();
    if (!pending) return;

    hasAutoSubmitted.current = true;
    reset(pending);
    setTimeout(() => {
      void handleSubmit(onSubmit)();
    }, 0);
  }, [auth, handleSubmit, onSubmit, reset]);

  return (
    <section className="space-y-6 rounded-3xl border border-slate-100 bg-white p-6 shadow-lg">
      <header className="space-y-3">
        <h1 className="text-xl font-semibold text-slate-900">アンケートを投稿する</h1>
        <div className="space-y-2 rounded-2xl border border-slate-200 bg-slate-50 p-4 text-xs text-slate-600">
          <p className="flex items-center gap-2 text-sm font-semibold text-slate-700">
            <span className="inline-flex h-6 w-6 items-center justify-center rounded-full bg-black font-semibold text-white">𝕏</span>
            アンケートの審査後に報酬を 𝕏 のDMでお送りします (PayPay 1,000 円分)
          </p>
        </div>
      </header>

      <form className="space-y-5" onSubmit={handleSubmit(onSubmit)}>
        <Field label="店舗名" required error={errors.storeName?.message}>
          <input
            id="storeName"
            type="text"
            placeholder="例: やりすぎ娘"
            {...register('storeName', { required: '店舗名は必須です' })}
            className="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:ring-2 focus:ring-pink-100 focus:outline-none"
          />
        </Field>

        <Field label="支店名" error={errors.branchName?.message}>
          <input
            id="branchName"
            type="text"
            placeholder="例: 新宿店"
            {...register('branchName')}
            className="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:ring-2 focus:ring-pink-100 focus:outline-none"
          />
        </Field>

        <Field label="都道府県" required error={errors.prefecture?.message}>
          <select
            id="prefecture"
            {...register('prefecture', { required: '都道府県を選択してください' })}
            className="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:ring-2 focus:ring-pink-100 focus:outline-none"
          >
            <option value="">選択してください</option>
            {PREFECTURES.map((prefecture) => (
              <option key={prefecture} value={prefecture}>
                {prefecture}
              </option>
            ))}
          </select>
        </Field>

        <Field label="業種" required error={errors.category?.message}>
          <select
            id="category"
            {...register('category', { required: '業種を選択してください' })}
            className="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:ring-2 focus:ring-pink-100 focus:outline-none"
          >
            <option value="">選択してください</option>
            {REVIEW_CATEGORIES.map((category) => (
              <option key={category.value} value={category.value}>
                {category.label}
              </option>
            ))}
          </select>
        </Field>

        <Field label="働いた時期" required error={errors.visitedAt?.message}>
          <input
            id="visitedAt"
            type="month"
            {...register('visitedAt', { required: '働いた時期を入力してください' })}
            className="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:ring-2 focus:ring-pink-100 focus:outline-none"
          />
        </Field>

        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="年齢" required error={errors.age?.message}>
            <select
              id="age"
              {...register('age', { valueAsNumber: true, required: '年齢を選択してください' })}
              className="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:ring-2 focus:ring-pink-100 focus:outline-none"
            >
              {AGE_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </Field>

          <Field label="スペック" required error={errors.specScore?.message}>
            <Controller
              name="specScore"
              control={control}
              rules={{ required: 'スペックを選択してください' }}
              render={({ field }) => (
                <div className="space-y-2">
                  <input
                    id="specScore"
                    type="range"
                    min={SPEC_MIN}
                    max={SPEC_MAX}
                    step={1}
                    value={field.value}
                    onChange={(event) => field.onChange(Number(event.target.value))}
                    className="w-full accent-pink-500"
                  />
                  <div className="flex items-center justify-between text-xs text-slate-500">
                    <span>{SPEC_MIN_LABEL}</span>
                    <span className="text-sm font-semibold text-slate-700">
                      {formatSpecScoreLabel(field.value)}
                    </span>
                    <span>{SPEC_MAX_LABEL}</span>
                  </div>
                </div>
              )}
            />
          </Field>

          <Field label="待機時間" required error={errors.waitTimeHours?.message}>
            <select
              id="waitTimeHours"
              {...register('waitTimeHours', {
                valueAsNumber: true,
                required: '待機時間を選択してください',
              })}
              className="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:ring-2 focus:ring-pink-100 focus:outline-none"
            >
              {WAIT_TIME_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </Field>

          <Field label="平均稼ぎ" required error={errors.averageEarning?.message}>
            <select
              id="averageEarning"
              {...register('averageEarning', {
                valueAsNumber: true,
                required: '平均稼ぎを選択してください',
              })}
              className="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:ring-2 focus:ring-pink-100 focus:outline-none"
            >
              {AVERAGE_EARNING_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </Field>
        </div>

        <Field
          label="客層・スタッフ・環境等について"
          error={errors.comment?.message}
        >
          <textarea
            id="comment"
            rows={5}
            placeholder="客層・スタッフ対応・待機環境など、気付いたことを自由にご記入ください"
            {...register('comment')}
            className="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:ring-2 focus:ring-pink-100 focus:outline-none"
          />
        </Field>

        <Field label="連絡用メールアドレス（任意）" error={errors.contactEmail?.message}>
          <div className="space-y-1">
            <input
              id="contactEmail"
              type="email"
              placeholder="例: example@example.com"
              {...register('contactEmail', {
                validate: (value) => {
                  const trimmed = value.trim();
                  if (!trimmed) return true;
                  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(trimmed) || '正しいメールアドレスを入力してください';
                },
              })}
              className="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:ring-2 focus:ring-pink-100 focus:outline-none"
            />
            <p className="text-xs text-slate-500">
              報酬連絡の控えとして利用します。入力がなくても投稿できます。
            </p>
          </div>
        </Field>

        <Field label="満足度" required error={errors.rating?.message}>
          <Controller
            name="rating"
            control={control}
            rules={{
              validate: (value) =>
                value >= RATING_MIN && value <= RATING_MAX
                  ? true
                  : '満足度を0〜5の範囲で選択してください',
            }}
            render={({ field }) => (
              <div className="space-y-3">
                <div className="flex items-center gap-3">
                  <StarDisplay value={field.value} />
                  <span className="text-sm text-slate-600">
                    {field.value.toFixed(1)} / {RATING_MAX.toFixed(1)}
                  </span>
                </div>
                <input
                  id="rating"
                  type="range"
                  min={RATING_MIN}
                  max={RATING_MAX}
                  step={RATING_STEP}
                  value={field.value}
                  onChange={(event) => field.onChange(Number(event.target.value))}
                  className="w-full accent-pink-500"
                />
                <div className="flex justify-between text-xs text-slate-500">
                  <span>0</span>
                  <span>2.5</span>
                  <span>5.0</span>
                </div>
              </div>
            )}
          />
        </Field>

        <div className="space-y-2 rounded-2xl bg-slate-50 p-4 text-xs text-slate-500">
          <p>投稿が完了すると、登録のTwitterアカウント宛に審査完了後のご案内をDMでお送りします。</p>
          <p>虚偽の情報が含まれる場合、掲載を停止することがあります。</p>
        </div>

        {status === 'success' ? (
          <p className="rounded-xl bg-emerald-50 px-4 py-3 text-sm text-emerald-700">
            投稿ありがとうございます！運営チームが内容を確認後、TwitterのDMで特典のご案内をお送りします。
          </p>
        ) : null}

        {status === 'error' && errorMessage ? (
          <p className="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700">{errorMessage}</p>
        ) : null}

        <button
          type="submit"
          disabled={status === 'submitting'}
          className="inline-flex w-full items-center justify-center rounded-full bg-gradient-to-r from-pink-500 to-violet-500 px-4 py-3 text-sm font-semibold text-white shadow-lg transition hover:from-pink-400 hover:to-violet-400 disabled:cursor-not-allowed disabled:opacity-60"
        >
          {status === 'submitting' ? '送信中...' : '投稿する'}
        </button>
      </form>

      {showSuccessModal ? (
        <div className="fixed inset-0 z-[210] flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-sm rounded-2xl bg-white p-6 shadow-2xl">
            <div className="space-y-3 text-center">
              <h2 className="text-lg font-semibold text-slate-900">投稿を受け付けました</h2>
              <p className="text-sm text-slate-600">
                アンケートありがとうございます！内容を審査後、𝕏 の DM（@
                {auth?.twitterUser?.username ?? '---'}）へ PayPay 1,000 円分のリンクをご案内します。
              </p>
              <p className="text-xs text-slate-400">
                審査には最大で 2〜3 営業日ほどお時間をいただく場合があります。
              </p>
            </div>
            <button
              type="button"
              className="mt-6 w-full rounded-full bg-gradient-to-r from-pink-500 to-violet-500 px-4 py-2 text-sm font-semibold text-white transition hover:from-pink-400 hover:to-violet-400"
              onClick={() => setShowSuccessModal(false)}
            >
              OK
            </button>
          </div>
        </div>
      ) : null}
    </section>
  );
};

type FieldProps = {
  label: string;
  required?: boolean;
  children: React.ReactNode;
  error?: string;
};

const Field = ({ label, required, children, error }: FieldProps) => {
  return (
    <div className="space-y-1 text-sm">
      <label className="font-semibold text-slate-700">
        {label}
        {required ? <span className="ml-1 text-pink-600">*</span> : null}
      </label>
      {children}
      {error ? <p className="text-xs text-red-600">{error}</p> : null}
    </div>
  );
};
