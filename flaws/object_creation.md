## 🧱 Object Creation Strategy

### ❓ Do We Really Need to Preallocate All Objects?

#### 💡 Insight

Just because the pool has an **initial** or **maximum capacity** of `X` objects doesn't mean we must eagerly create all `X` objects upfront.

#### 🔍 Key Principles

- **Capacity** defines the _upper bound_ — not a requirement to prefill.
- **Lazy instantiation** allows the pool (or fast path) to defer object creation until actual demand arises.
- Instead of preallocating, the pool can **track how many objects have been created so far**, and create new ones **only when needed**, up to the buffer’s limit.

> **Growing** behaves similarly to initialization — it creates objects up to the new capacity limit. But this isn’t necessary; instead of allocating all objects at once (which can be wasteful), we should grow in controlled batches — not one at a time, but not all at once either.

#### ⚙️ Implications for Design

- Avoid unnecessary memory pressure during startup.
- Allow capacity to be treated as a _soft limit_, not a _target_.
