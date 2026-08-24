export interface CustomConfigEntry {
  id: string;
  comments: string[];
  command: string;
}

export interface ParsedCustomConfig {
  entries: CustomConfigEntry[];
  trailingComments: string[];
  trailingStartLine: number;
}

let customConfigEntrySequence = 0;

const nextCustomConfigEntryId = () => {
  customConfigEntrySequence += 1;
  return `server-config-${Date.now()}-${customConfigEntrySequence}`;
};

export const parseCustomConfigText = (text: string): ParsedCustomConfig =>
  parseCustomConfigLines(text.replace(/\r\n?/g, '\n').split('\n'));

export const parseCustomConfigLines = (lines: string[]): ParsedCustomConfig => {
  const entries: CustomConfigEntry[] = [];
  let pendingComments: string[] = [];
  let pendingStartLine = 0;

  lines.forEach((rawLine, index) => {
    const line = rawLine.replace(/\r$/, '');
    const trimmed = line.trim();
    if (!trimmed) return;

    if (trimmed.startsWith('//')) {
      if (pendingComments.length === 0) pendingStartLine = index + 1;
      pendingComments.push(trimmed.slice(2).trim());
      return;
    }

    const { command, comment, hasComment } = splitInlineComment(line);
    const normalizedCommand = command.trim();
    if (!normalizedCommand) return;

    const comments = [...pendingComments];
    if (hasComment) comments.push(comment);
    entries.push({
      id: nextCustomConfigEntryId(),
      comments,
      command: normalizedCommand,
    });
    pendingComments = [];
    pendingStartLine = 0;
  });

  return {
    entries,
    trailingComments: [...pendingComments],
    trailingStartLine: pendingStartLine,
  };
};

export const serializeCustomConfigEntries = (entries: CustomConfigEntry[]): string[] => {
  const lines: string[] = [];
  entries.forEach((entry) => {
    entry.comments.forEach((comment) => {
      const normalizedComment = comment.trim();
      lines.push(normalizedComment ? `// ${normalizedComment}` : '//');
    });
    const command = entry.command.trim();
    if (command) lines.push(command);
  });
  return lines;
};

const splitInlineComment = (line: string) => {
  let inQuotes = false;
  for (let index = 0; index + 1 < line.length; index += 1) {
    if (line[index] === '"' && !isEscaped(line, index)) {
      inQuotes = !inQuotes;
      continue;
    }
    if (inQuotes || line[index] !== '/' || line[index + 1] !== '/') continue;
    if (isUrlSlash(line, index)) continue;
    return {
      command: line.slice(0, index).trim(),
      comment: line.slice(index + 2).trim(),
      hasComment: true,
    };
  }
  return { command: line, comment: '', hasComment: false };
};

const isUrlSlash = (line: string, slashIndex: number) => {
  let tokenStart = slashIndex;
  while (tokenStart > 0 && line[tokenStart - 1] !== ' ' && line[tokenStart - 1] !== '\t') {
    tokenStart -= 1;
  }
  const tokenPrefix = line.slice(tokenStart, slashIndex);
  if (tokenPrefix.includes('"')) return false;
  let colonIndex = tokenPrefix.lastIndexOf('://');
  if (colonIndex < 0) {
    if (!tokenPrefix.endsWith(':')) return false;
    colonIndex = tokenPrefix.length - 1;
  }

  let schemeStart = colonIndex;
  while (schemeStart > 0 && /[A-Za-z0-9+.-]/.test(tokenPrefix[schemeStart - 1] ?? '')) {
    schemeStart -= 1;
  }
  const scheme = tokenPrefix.slice(schemeStart, colonIndex);
  return /^[A-Za-z][A-Za-z0-9+.-]*$/.test(scheme);
};

const isEscaped = (value: string, index: number) => {
  let backslashes = 0;
  for (let current = index - 1; current >= 0 && value[current] === '\\'; current -= 1) {
    backslashes += 1;
  }
  return backslashes % 2 === 1;
};
