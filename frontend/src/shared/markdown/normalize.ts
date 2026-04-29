const HORIZONTAL_RULE_REGEX = /^(\s*)(-{3,}|\*{3,}|_{3,})\s*$/;
const FENCE_REGEX = /^\s*(```+|~~~+)/;
const INDENTED_CODE_REGEX = /^(\s{4,}|\t)/;
const BLOCKQUOTE_REGEX = /^(\s*(?:>\s*)+)(.*)$/;

const normalizeLine = (line: string): string => {
  if (!line) {
    return line;
  }

  if (INDENTED_CODE_REGEX.test(line)) {
    return line;
  }

  if (HORIZONTAL_RULE_REGEX.test(line.trim())) {
    return line;
  }

  let prefix = "";
  let body = line;
  const blockquoteMatch = line.match(BLOCKQUOTE_REGEX);
  if (blockquoteMatch) {
    prefix = blockquoteMatch[1] ?? "";
    body = blockquoteMatch[2] ?? "";
  }

  let normalized = body;
  normalized = normalized.replace(/^(\s{0,3})(#{1,6})(?!#)(\S)/, "$1$2 $3");
  normalized = normalized.replace(/^(\s*)([*+-])(?=\S)/, "$1$2 ");
  normalized = normalized.replace(/^(\s*)(\d+[.)])(?=\S)/, "$1$2 ");
  normalized = normalized.replace(
    /^(\s*)([*+-]|\d+[.)])\s*(\[(?:[xX ]?)\])(?=\S)/,
    "$1$2 $3 "
  );

  return `${prefix}${normalized}`;
};

export const normalizeMarkdown = (text: string): string => {
  if (!text) {
    return text;
  }

  const lines = text.split("\n");
  let inFence = false;
  let fenceChar: "`" | "~" | null = null;

  const normalizedLines = lines.map((line) => {
    const fenceMatch = line.match(FENCE_REGEX);
    if (fenceMatch) {
      const currentChar = fenceMatch[1]?.[0] === "~" ? "~" : "`";
      if (!inFence) {
        inFence = true;
        fenceChar = currentChar;
      } else if (fenceChar === currentChar) {
        inFence = false;
        fenceChar = null;
      }
      return line;
    }

    if (inFence) {
      return line;
    }

    return normalizeLine(line);
  });

  return normalizedLines.join("\n");
};
