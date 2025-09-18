// Logger simple avec heure et niveau
function getTimestamp() {
  return new Date().toISOString();
}

// n'activer les logs que si NODE_ENV=development
const isDev = process.env.NODE_ENV === 'development';

function log(level, ...args) {
  if (!isDev) return; // no-op en production
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

export default logger;
