export function parseMask(mask) {
  if (typeof mask !== 'string') throw new TypeError('mask must be a string');
  if (!/^[01\*]{3}$/.test(mask)) throw new Error('Invalid mask format');
  const [d, m, y] = mask.split('');
  const toVal = c => (c === '1' ? true : c === '0' ? false : null);
  return { daily: toVal(d), monthly: toVal(m), yearly: toVal(y) };
}

// Apply mask to existing flags. existing may be object with booleans, or undefined.
// '*' (null) keeps existing; if existing undefined, null -> false
export function applyMask(existing = { daily: false, monthly: false, yearly: false }, mask) {
  const parsed = parseMask(mask);
  return {
    daily: parsed.daily === null ? !!existing.daily : parsed.daily,
    monthly: parsed.monthly === null ? !!existing.monthly : parsed.monthly,
    yearly: parsed.yearly === null ? !!existing.yearly : parsed.yearly,
  };
}

export default { parseMask, applyMask };
