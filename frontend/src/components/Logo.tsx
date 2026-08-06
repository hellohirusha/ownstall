import { useId } from "react";
import { Link } from "react-router-dom";

// Same geometry as the generated PNG/ICO set (see the icon generator): a
// near-black rounded square with a thick white O. Stroke weight and ground
// colour are carried over from the previous mark.
//
// Drawn on a 48-unit grid with the ring punched by a mask so the counter stays
// a true hole on any background.
const RING_CX = 24;
const RING_CY = 24;
const RING_R_OUT = 14.2;
const RING_R_IN = 8.4;

export function LogoMark({
  size = 32,
  className = "",
}: {
  size?: number;
  className?: string;
}) {
  // Several marks can render on one page (header, footer, storefront), and
  // duplicate mask ids would make them reference each other's mask.
  const maskId = `ownstall-mark-${useId()}`;

  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 48 48"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={className}
      role="img"
      aria-label="Ownstall"
    >
      <mask id={maskId}>
        <rect width="48" height="48" fill="black" />
        <circle cx={RING_CX} cy={RING_CY} r={RING_R_OUT} fill="white" />
        <circle cx={RING_CX} cy={RING_CY} r={RING_R_IN} fill="black" />
      </mask>

      <rect width="48" height="48" rx="11" fill="#111111" />
      <rect width="48" height="48" fill="#ffffff" mask={`url(#${maskId})`} />
    </svg>
  );
}

interface LogoProps {
  /** Renders as a link to the homepage unless this is false. */
  linked?: boolean;
  size?: number;
  /** Tailwind text colour for the wordmark. */
  className?: string;
}

export function Logo({
  linked = true,
  size = 32,
  className = "text-ink-900",
}: LogoProps) {
  const content = (
    <span className="inline-flex items-center gap-2">
      <LogoMark size={size} />
      <span
        className={`font-bold tracking-tight ${className}`}
        style={{ fontSize: size * 0.62 }}
      >
        Ownstall
      </span>
    </span>
  );

  if (!linked) return content;

  return (
    <Link to="/" className="inline-flex items-center" aria-label="Ownstall home">
      {content}
    </Link>
  );
}
