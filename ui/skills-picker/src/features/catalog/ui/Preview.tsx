import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { FileText, X } from '../../../shared/ui/icons';
import { usePreview } from '../api/queries';
import { Busy, Problem } from '../../../shared/ui/primitives';

function externalUrl(value: string) {
  try { const url = new URL(value); return ['https:', 'http:'].includes(url.protocol) ? url.href : ''; } catch { return ''; }
}
export function Preview({ sessionId, catalogId, candidateId, close }: { sessionId: string; catalogId: string; candidateId?: string; close: () => void }) {
  const preview = usePreview(sessionId, catalogId, candidateId);
  return <section className="preview" aria-label="Skill preview">
    <div className="panel-heading"><h2><FileText size={16}/> Skill preview</h2>{candidateId && <button className="icon-button" aria-label="Close preview" onClick={close}><X size={16}/></button>}</div>
    {!candidateId ? <div className="preview-placeholder"><FileText size={32}/><h3>A closer look</h3><p>Open a skill to read its instructions before adding it.</p></div> :
      preview.isPending ? <Busy>Loading preview…</Busy> : preview.isError ? <Problem error={preview.error} retry={() => void preview.refetch()}/> :
        preview.data?.catalogId === catalogId && preview.data.candidateId === candidateId && <>
          <small className="path">{preview.data.path || '/'}</small>
          {preview.data.truncated && <div className="notice">Preview shortened. Installation checks the complete skill.</div>}
          <div className="markdown">{preview.data.format === 'text' ? <pre>{preview.data.content}</pre> : <Markdown skipHtml remarkPlugins={[remarkGfm]} urlTransform={externalUrl} components={{
            img: ({ alt }) => <span className="image-placeholder">[Image: {alt || 'not loaded'}]</span>,
            a: ({ href, children }) => href ? <a href={href} target="_blank" rel="noopener noreferrer" referrerPolicy="no-referrer">{children}</a> : <span>{children}</span>,
          }}>{preview.data.content}</Markdown>}</div>
          <details className="raw"><summary>View raw instructions</summary><pre>{preview.data.rawContent ?? preview.data.content}</pre></details>
        </>}
  </section>;
}
