/* eslint-disable @typescript-eslint/no-unsafe-member-access */
/* eslint-disable @typescript-eslint/no-unsafe-call */
// @ts-expect-error xbbcode doesn't have type definitions
import XBBCode from 'xbbcode';

interface BBTag {
  getAttribute(attr: string): string;
  getContent(): string;
  getOption(): string | null;
}

const parser = XBBCode.create({
  b: '<strong>{content}</strong>',
  i: '<em>{content}</em>',
  u: '<u>{content}</u>',
  s: '<s>{content}</s>',
  list: (tag: BBTag) => {
    const name = tag.getAttribute('type') === 'decimal' ? 'ol' : 'ul';
    return '<' + name + '>' + tag.getContent() + '</' + name + '>';
  },
  li: '<li>{content}</li>',
  font: '<span style="font-family: {option}">{content}</span>',
  size: '<span style="font-size: {option}">{content}</span>',
  color: '<span style="color: {option}">{content}</span>',
  img: '<img src="{option}">',
  url: (tag: BBTag) => {
    const target = (tag.getOption() || tag.getContent() || '#').replace(/"/g, '&quot;');
    return `<a href="${target}" class="open-ext">${tag.getContent()}</a>`;
  },
  sup: '<sup>{content}</sup>',
  sub: '<sub>{content}</sub>',
  tt: '<tt>{content}</tt>',
  pre: '<pre>{content}</pre>',
  left: '<div style="text-align: left;">{content}</div>',
  right: '<div style="text-align: right;">{content}</div>',
  center: '<div style="text-align: center;">{content}</div>',
  table: '<table class="table">{content}</table>',
  tr: '<tr>{content}</tr>',
  td: '<td>{content}</td>',
  code: '<pre>{content}</pre>',
  quote: '<blockquote>{content}</blockquote>',
  yt:
    '<iframe width="640" height="385" allowfullscreen src="https://www.youtube-nocookie.com/embed/{content}?version=3&vq=hd720"></iframe>',
  p3d: '<iframe width="640" height="385" allowfullscreen src="http://p3d.in/e/{content}"></iframe>',
}) as (input: string) => string;

export default (text: string): string => {
  text = text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/\]\s*\n+\s*\[/g, '][')
    .replace(/\n/g, '<br>');
  text = text.replace(/\[hr\]/g, '<hr>').replace(/\[br\]/g, '<br>');
  return parser(text);
};
