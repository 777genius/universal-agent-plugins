import MarkdownIt from 'markdown-it';

// Publisher text is untrusted. No raw HTML, remote images, generated IDs,
// relative links, or executable URL schemes enter the rendered document.
const markdown = new MarkdownIt({
  html: false,
  linkify: false,
  typographer: true,
  breaks: false,
});
markdown.disable('image');
markdown.validateLink = (href) => {
  try {
    const url = new URL(href);
    const hostname = url.hostname.toLowerCase();
    return (
      (url.protocol === 'https:' || url.protocol === 'http:') &&
      !url.username &&
      !url.password &&
      hostname !== 'skills.sh' &&
      !hostname.endsWith('.skills.sh')
    );
  } catch {
    return false;
  }
};
markdown.renderer.rules.link_open = (tokens, index, options, env, self) => {
  tokens[index]!.attrSet('target', '_blank');
  tokens[index]!.attrSet('rel', 'noopener noreferrer nofollow');
  return self.renderToken(tokens, index, options);
};

export function skillDocumentBody(raw: string): string {
  if (!raw.startsWith('---\n') && !raw.startsWith('---\r\n')) return raw;
  const end = /^---\r?\n[\s\S]*?\r?\n---(?:\r?\n|$)/.exec(raw);
  return end ? raw.slice(end[0].length).trimStart() : raw;
}

export function renderSkillMarkdown(raw: string): string {
  // The public document API caps source bytes at 64 KiB.
  return markdown.render(skillDocumentBody(raw.slice(0, 64 << 10)));
}
