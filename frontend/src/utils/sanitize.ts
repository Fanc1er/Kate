import DOMPurify from 'dompurify'

DOMPurify.addHook('uponSanitizeAttribute', (_node, data) => {
  // 允许 img 的 src 协议为 http/https/data（图片型证据），其余标签的 href 限制 http/https。
  if (data.attrName === 'src' || data.attrName === 'href') {
    const val = String(data.attrValue ?? '')
    if (val.startsWith('http:') || val.startsWith('https:') || val.startsWith('data:image/')) {
      return
    }
    data.attrValue = ''
  }
})

export function sanitizeHtml(html: string): string {
  return DOMPurify.sanitize(html, {
    ALLOWED_TAGS: [
      'p', 'div', 'span', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
      'ul', 'ol', 'li', 'a', 'img', 'pre', 'code', 'blockquote',
      'strong', 'em', 'b', 'i', 'u', 'br', 'hr', 'table', 'thead',
      'tbody', 'tr', 'th', 'td', 'input', 'form',
    ],
    ALLOWED_ATTR: ['href', 'src', 'alt', 'title', 'target', 'rel', 'width', 'height', 'style'],
  })
}
