---
notion-id: 2fe57008-e885-819e-a0f9-c28e89f783fc
notion-url: "https://www.notion.so/Code-Math-Quotes-2fe57008e885819ea0f9c28e89f783fc"
notion-frozen-at: "2026-02-10T17:03:13Z"
notion-last-edited: "2026-02-05T19:44:00.000Z"
notion-database-id: 2fe57008-e885-8003-b1f3-cc05981dc6b0
Approved: true
Assignee: []
Attachments: []
Category: Design
"Contact Email": carol@example.com
Created: "2026-02-05T19:44:00.000Z"
Description: Tests code blocks, equations, quotes, and callouts
"Due Date": 2026-01-10
"Last Edited": "2026-02-05T19:44:00.000Z"
Phone: +81-3-1234-5678
Related: []
Score: 78.5
Tags:
  - docs
Website: "https://example.com/design"
---
## Code Blocks
```python
def fibonacci(n):
    if n <= 1:
        return n
    return fibonacci(n - 1) + fibonacci(n - 2)

print(fibonacci(10))
```
```javascript
const greet = (name) => {
  console.log(`Hello, ${name}!`);
};
greet('World');
```
```sql
SELECT u.name, COUNT(o.id) AS order_count
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
GROUP BY u.name
HAVING COUNT(o.id) > 5;
```
```javascript
Plain text code block with no language specified.
Line two of plain text.
```
## Block Equations
$$
\int_{0}^{\infty} e^{-x^2} dx = \frac{\sqrt{\pi}}{2}
$$
$$
\sum_{n=1}^{\infty} \frac{1}{n^2} = \frac{\pi^2}{6}
$$
## Quotes
> This is a simple blockquote with some **bold** and *italic* text.
> A longer quote that discusses important concepts.
> It spans multiple lines using line breaks.
> And has a third line for good measure.
## Callouts
> [!tip]
> This is a tip callout with useful information.
> [!warning]
> Warning: This operation cannot be undone!
> [!danger]
> Danger! Critical system failure detected.
> [!note]
> Note: Remember to update the documentation after changes.
> - Item inside callout
> - Another item