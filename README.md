# hack-kit

Процесс для хакатонов и проектов с агентами Claude: изменения через PR, согласование меткой, ревью в CI, исполнители в routines. Собран после UMIRHack 2026.

- Что это и почему так — [hack-kit/README.md](hack-kit/README.md), [hack-kit/LESSONS.md](hack-kit/LESSONS.md)
- Процесс — [hack-kit/PROCESS.md](hack-kit/PROCESS.md), первый день — [hack-kit/DAY0.md](hack-kit/DAY0.md)
- Как общаются агенты и как их настроить — [hack-kit/AGENTS-SETUP.md](hack-kit/AGENTS-SETUP.md)

Корень репозитория — сам шаблон (`AGENTS.md`, `.agents/skills/`, `.github/`, `changes/`, `docs/`, `frontend/`, `backend/`) с маленькой заглушкой проекта: на ней проверяется цепочка «предложение → `go:<id>` → исполнитель → ревью → слияние».
