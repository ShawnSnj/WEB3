# Versioned migration history

Incremental `000N_*.up.sql` / `000N_*.down.sql` files live here for
reference. **Deploy using the single file at the parent level:**

```bash
make migrate          # dockerised postgres
psql $DATABASE_URL -f migrations/deploy.sql   # any postgres
```

`deploy.sql` is the canonical, idempotent schema. These versioned files
are kept to document how the schema evolved step-by-step.
