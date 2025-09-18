export function parseMask(mask) {
  // Nouveau format: 2 caractères (monthly, yearly) -> 'my'
  if (typeof mask !== 'string') throw new TypeError('mask must be a string');
  if (!/^[01\*]{2}$/.test(mask)) throw new Error('Invalid mask format (expected 2 chars: monthly,yearly)');
  const [m, y] = mask.split('');
  const toVal = c => (c === '1' ? true : c === '0' ? false : null);
  // daily n'est plus configurable : toujours false. On garde la propriété pour compatibilité.
  return { daily: false, monthly: toVal(m), yearly: toVal(y) };
}

// Apply mask to existing flags. existing may be object with booleans, or undefined.
// '*' (null) keeps existing; if existing undefined, null -> false
export function applyMask(existing = { daily: false, monthly: false, yearly: false }, mask) {
  const parsed = parseMask(mask);
  return {
    daily: false, // daily disabled
    monthly: parsed.monthly === null ? !!existing.monthly : parsed.monthly,
    yearly: parsed.yearly === null ? !!existing.yearly : parsed.yearly,
  };
}

export default { parseMask, applyMask };
