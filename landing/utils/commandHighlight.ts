export interface HighlightToken {
  text: string;
  className: string;
}

export type HighlightedCommandLine = HighlightToken[];

const commandActions = new Set([
  'install',
  'version',
  'init',
  'generate',
  'validate',
  'run',
  'open',
  'link',
  'config',
  'disable',
  'enable',
  'publish',
  'fetch',
  'integrations',
  'add',
  'update',
  'repair',
  'remove',
  'sync',
]);

const classifyToken = (token: string, tokenIndex: number): string => {
  if (['|', '&&', '||'].includes(token)) {
    return 'operator';
  }

  if (token.startsWith('https://') || token.startsWith('http://')) {
    return 'url';
  }

  if (token.startsWith('--') || (token.startsWith('-') && token.length > 1)) {
    return 'flag';
  }

  if (tokenIndex === 0) {
    return 'command';
  }

  if (tokenIndex === 1 && commandActions.has(token)) {
    return 'action';
  }

  if (
    token === '.' ||
    token.startsWith('./') ||
    token.startsWith('/') ||
    token.startsWith('~/') ||
    token.includes('/') ||
    token.endsWith('.sh') ||
    token.endsWith('.yaml') ||
    token.endsWith('.json') ||
    token.endsWith('.txt')
  ) {
    return 'path';
  }

  return 'plain';
};

export const renderHighlightedCommand = (command: string): HighlightedCommandLine[] =>
  command.split('\n').map((line) => {
    const tokens = line.match(/\S+|\s+/g) || [];
    let tokenIndex = 0;

    return tokens.map((part) => {
      if (/^\s+$/.test(part)) {
        return { text: part, className: 'plain' };
      }

      const tokenClass = classifyToken(part, tokenIndex);
      tokenIndex += 1;
      return { text: part, className: tokenClass };
    });
  });
