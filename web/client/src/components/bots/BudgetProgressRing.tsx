import { useMemo } from 'react';
import { cn } from '@/lib/utils';

interface BudgetProgressRingProps {
  /** 0-1 fraction of budget consumed */
  value: number;
  /** Display label inside the ring */
  label: string;
  /** Secondary text below the label */
  sublabel?: string;
  /** Ring size in pixels */
  size?: number;
  /** Ring stroke width */
  strokeWidth?: number;
  className?: string;
}

/**
 * Apple-style circular progress ring with gradient stroke.
 * Green -> Amber -> Red as value approaches 1.0.
 */
export function BudgetProgressRing({
  value,
  label,
  sublabel,
  size = 120,
  strokeWidth = 8,
  className,
}: BudgetProgressRingProps) {
  const clamped = Math.min(Math.max(value, 0), 1);

  const { circumference, offset, gradientId, color } = useMemo(() => {
    const radius = (size - strokeWidth) / 2;
    const circ = 2 * Math.PI * radius;
    const off = circ * (1 - clamped);
    const id = `ring-${Math.random().toString(36).slice(2, 9)}`;

    // Color stops: green < 0.6, amber 0.6-0.85, red > 0.85
    let c: { start: string; end: string; text: string };
    if (clamped < 0.6) {
      c = { start: '#34d399', end: '#10b981', text: 'text-emerald-400' };
    } else if (clamped < 0.85) {
      c = { start: '#fbbf24', end: '#f59e0b', text: 'text-amber-400' };
    } else {
      c = { start: '#f87171', end: '#ef4444', text: 'text-red-400' };
    }

    return { circumference: circ, offset: off, gradientId: id, color: c };
  }, [clamped, size, strokeWidth]);

  const radius = (size - strokeWidth) / 2;
  const center = size / 2;

  return (
    <div className={cn('relative inline-flex items-center justify-center', className)}>
      <svg
        width={size}
        height={size}
        viewBox={`0 0 ${size} ${size}`}
        className="transform -rotate-90"
      >
        <defs>
          <linearGradient id={gradientId} x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stopColor={color.start} />
            <stop offset="100%" stopColor={color.end} />
          </linearGradient>
        </defs>

        {/* Background track */}
        <circle
          cx={center}
          cy={center}
          r={radius}
          fill="none"
          stroke="currentColor"
          strokeWidth={strokeWidth}
          className="text-muted/30"
        />

        {/* Progress arc */}
        <circle
          cx={center}
          cy={center}
          r={radius}
          fill="none"
          stroke={`url(#${gradientId})`}
          strokeWidth={strokeWidth}
          strokeDasharray={circumference}
          strokeDashoffset={offset}
          strokeLinecap="round"
          className="transition-all duration-700 ease-out"
        />
      </svg>

      {/* Center text */}
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className={cn('text-lg font-semibold leading-none tracking-tight', color.text)}>
          {label}
        </span>
        {sublabel && (
          <span className="text-[10px] text-text-tertiary mt-1 leading-none">
            {sublabel}
          </span>
        )}
      </div>
    </div>
  );
}
