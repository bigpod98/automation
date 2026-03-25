'use client';

import { useState } from 'react';

interface ItemListProps {
  label: string;
  items: string[];
  count: number;
}

export function ItemList({ label, items, count }: ItemListProps) {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className="space-y-1.5">
      <button
        onClick={() => count > 0 && setExpanded((e) => !e)}
        className="flex items-center justify-between w-full text-left"
        disabled={count === 0}
      >
        <span className="text-xs font-medium text-slate-400 uppercase tracking-wider">
          {label}
        </span>
        <div className="flex items-center gap-1.5">
          <span
            className={`text-xl font-bold tabular-nums leading-none ${
              count > 0 ? 'text-white' : 'text-slate-600'
            }`}
          >
            {count}
          </span>
          {count > 0 && (
            <svg
              className={`w-3 h-3 text-slate-500 transition-transform duration-150 ${
                expanded ? 'rotate-180' : ''
              }`}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M19 9l-7 7-7-7"
              />
            </svg>
          )}
        </div>
      </button>

      {expanded && count > 0 && (
        <ul className="space-y-0.5 pl-2 border-l-2 border-slate-700">
          {items.map((item) => (
            <li
              key={item}
              className="text-xs text-slate-300 truncate py-0.5 leading-relaxed"
              title={item}
            >
              {item}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
