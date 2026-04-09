import { useState } from 'react';

type FormState = 'IDLE' | 'LOADING' | 'SUCCESS';

export default function WaitlistForm() {
  const [state, setState] = useState<FormState>('IDLE');
  const [email, setEmail] = useState('');
  const [hasError, setHasError] = useState(false);

  const isValidEmail = (v: string) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v);

  function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!isValidEmail(email)) { setHasError(true); return; }
    setHasError(false);
    setState('LOADING');
    setTimeout(() => { console.log(email); setState('SUCCESS'); }, 1000);
  }

  if (state === 'SUCCESS') {
    return (
      <div className="flex items-center gap-2 text-sm font-medium" style={{ color: 'var(--color-green)' }}>
        <svg className="w-4 h-4" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
        </svg>
        Registrado correctamente
      </div>
    );
  }

  return (
    <div>
      <form onSubmit={handleSubmit} noValidate className="flex gap-2 flex-wrap justify-center">
        <input
          type="email"
          value={email}
          onChange={e => { setEmail(e.target.value); if (hasError) setHasError(false); }}
          placeholder="tu@empresa.com"
          disabled={state === 'LOADING'}
          required
          style={{
            fontFamily: 'var(--font-sans)',
            fontSize: '14px',
            color: 'var(--color-text)',
            background: 'white',
            border: hasError ? '1px solid var(--color-red)' : '1px solid var(--color-border)',
            borderRadius: '8px',
            height: '42px',
            width: '100%',
            maxWidth: '260px',
            padding: '0 14px',
            outline: 'none',
          }}
          onFocus={e => { if (!hasError) e.currentTarget.style.borderColor = 'var(--color-accent)'; }}
          onBlur={e => { if (!hasError) e.currentTarget.style.borderColor = 'var(--color-border)'; }}
        />
        <button
          type="submit"
          disabled={state === 'LOADING'}
          style={{
            fontFamily: 'var(--font-sans)',
            fontSize: '14px',
            fontWeight: 500,
            color: 'white',
            background: 'var(--color-accent)',
            border: 'none',
            borderRadius: '8px',
            height: '42px',
            padding: '0 18px',
            cursor: state === 'LOADING' ? 'not-allowed' : 'pointer',
            opacity: state === 'LOADING' ? 0.7 : 1,
            transition: 'background 150ms ease',
          }}
          onMouseEnter={e => { if (state !== 'LOADING') e.currentTarget.style.background = 'var(--color-accent-hover)'; }}
          onMouseLeave={e => { if (state !== 'LOADING') e.currentTarget.style.background = 'var(--color-accent)'; }}
        >
          {state === 'LOADING' ? 'Enviando...' : 'Unirme'}
        </button>
      </form>
      {hasError && (
        <p style={{ fontSize: '12px', color: 'var(--color-red)', marginTop: '6px' }}>Verificá el formato del email.</p>
      )}
    </div>
  );
}
