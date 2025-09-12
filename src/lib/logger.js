// Logger simple avec heure et niveau
function getTimestamp() {
  return new Date().toISOString();
}

function log(level, ...args) {
  const timestamp = getTimestamp();
  const msg = args.map(a => (typeof a === 'object' ? JSON.stringify(a) : a)).join(' ');
  console.log(`[${timestamp}] [${level}] ${msg}`);
}

export const logger = {
  info: (...args) => log('INFO', ...args),
  warn: (...args) => log('WARN', ...args),
  error: (...args) => log('ERROR', ...args),
  debug: (...args) => log('DEBUG', ...args),
};
