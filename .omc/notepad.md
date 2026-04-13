# Notepad
<!-- Auto-managed by OMC. Manual edits preserved in MANUAL section. -->

## Priority Context
<!-- ALWAYS loaded. Keep under 500 chars. Critical discoveries only. -->
Nanobot Refactoring Plan:

Current: Project-based (per-project .cobot/, hashed memory dirs)
Target: Personal agent (~/.cobot/, global memory, nanobot-style)

Key Changes:
1. Remove workspace hashing - use ~/.cobot/memory/
2. Add SOUL.md, USER.md, MEMORY.md, HISTORY.md
3. Dual-layer memory: MEMORY.md (in prompt) + history.jsonl (searchable)
4. Wings become personal domains, not projects
5. Token budget-driven consolidation (not time-based)

Files to modify:
- internal/workspace/workspace.go (remove hashing)
- internal/workspace/discovery.go (make optional)
- internal/memory/ (add nanobot-style memory files)
- cmd/cobot/ (update paths)
- New: internal/persona/ (SOUL, USER, MEMORY management)

## Working Memory
<!-- Session notes. Auto-pruned after 7 days. -->
### 2026-04-12 08:11
Created Makefile with comprehensive build targets


## MANUAL
<!-- User content. Never auto-pruned. -->

