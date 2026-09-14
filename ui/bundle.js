// Gitea plugin UI bundle — source-control recipe, inlined as plain JS.
// Registers a repository provider, review provider, and task link action
// that delegate to the plugin backend through host.api.invokeAction.

function record(value) {
  return value !== null && typeof value === "object" ? value : {};
}
function text(value) {
  return typeof value === "string" ? value.trim() : "";
}
function finiteNumber(value) {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}
function nonNegativeInteger(value) {
  const n = finiteNumber(value);
  return n !== undefined && Number.isInteger(n) && n >= 0 ? n : undefined;
}
function positiveInteger(value) {
  const n = nonNegativeInteger(value);
  return n !== undefined && n > 0 ? n : undefined;
}

function createSnapshotStore() {
  const snapshots = new Map();
  const listeners = new Map();
  const versions = new Map();
  let epoch = 0;
  return {
    get: (key) => snapshots.get(key) ?? [],
    subscribe(key, listener) {
      const kl = listeners.get(key) ?? new Set();
      kl.add(listener);
      listeners.set(key, kl);
      return () => { kl.delete(listener); if (kl.size === 0) listeners.delete(key); };
    },
    beginRefresh(key) {
      const version = (versions.get(key) ?? 0) + 1;
      versions.set(key, version);
      return { epoch, version };
    },
    commit(key, token, values) {
      if (token.epoch !== epoch || versions.get(key) !== token.version) return false;
      snapshots.set(key, [...values]);
      const kl = listeners.get(key);
      if (kl) kl.forEach((fn) => fn());
      return true;
    },
    clear() {
      epoch += 1;
      versions.clear();
      snapshots.clear();
      listeners.forEach((kl) => kl.forEach((fn) => fn()));
      listeners.clear();
    },
  };
}

function normalizeTaskStatus(value) {
  const s = record(value);
  const number = positiveInteger(s.number);
  const state = text(s.state);
  const pipelineState = text(s.pipeline_state);
  if (number === undefined ||
    !["open", "merged", "closed", "draft"].includes(state) ||
    !["success", "failure", "pending", "neutral"].includes(pipelineState))
    return undefined;

  const checks = Array.isArray(s.checks) ? s.checks.flatMap((v) => {
    const c = record(v);
    const id = text(c.id), label = text(c.label), cs = text(c.state);
    if (!id || !label || !["success", "failure", "pending", "neutral"].includes(cs)) return [];
    const detail = text(c.detail), url = text(c.url);
    return [{ id, label, state: cs, ...(detail ? { detail } : {}), ...(url ? { url } : {}) }];
  }) : [];

  const rs = record(s.review);
  const reviewState = text(rs.state);
  const approved = nonNegativeInteger(rs.approved);
  const required = nonNegativeInteger(rs.required);
  const requested = nonNegativeInteger(rs.requested);
  const review = approved !== undefined && ["approved", "changes_requested", "pending"].includes(reviewState)
    ? { state: reviewState, approved, ...(required === undefined ? {} : { required }), ...(requested === undefined ? {} : { requested }) }
    : undefined;
  const unresolvedComments = nonNegativeInteger(s.unresolved_comments);
  const updatedAt = finiteNumber(s.updated_at);

  return {
    number, state, pipelineState, checks,
    ...(review ? { review } : {}),
    ...(unresolvedComments === undefined ? {} : { unresolvedComments }),
    ...(updatedAt === undefined ? {} : { updatedAt }),
  };
}

function normalizeReview(providerId, value) {
  const s = record(value);
  const reviewKey = text(s.review_key), title = text(s.title), url = text(s.url);
  const connectionScope = text(s.connection_scope), repositoryId = text(s.repository_id);
  const changeRequestNumber = positiveInteger(s.change_request_number);
  if (!reviewKey || !title || !url || !connectionScope || !repositoryId || changeRequestNumber === undefined) return null;
  const state = text(s.state);
  const taskStatus = normalizeTaskStatus(s.task_status);
  return { providerId, reviewKey, title, url, connectionScope, repositoryId, changeRequestNumber, state, ...(taskStatus ? { taskStatus } : {}) };
}

function normalizeAssociation(providerId, value) {
  const s = record(value);
  const taskId = text(s.task_id), reviewKey = text(s.review_key);
  const connectionScope = text(s.connection_scope), repositoryId = text(s.repository_id);
  const changeRequestNumber = positiveInteger(s.change_request_number);
  if (!taskId || !reviewKey || !connectionScope || !repositoryId || changeRequestNumber === undefined) return null;
  return { providerId, taskId, reviewKey, connectionScope, repositoryId, changeRequestNumber };
}

function normalizeRepository(providerId, value) {
  const s = record(value);
  const providerHost = text(s.provider_host), ownerOrProject = text(s.owner_or_project);
  const repositoryId = text(s.repository_id), repositoryName = text(s.name);
  const cloneUrl = text(s.clone_url);
  if (!providerHost || !ownerOrProject || !repositoryId || !repositoryName || !cloneUrl) return null;
  const providerScope = text(s.provider_scope);
  const defaultBranch = text(s.default_branch);
  return {
    providerId, providerHost,
    ...(providerScope ? { providerScope } : {}),
    ownerOrProject, repositoryId, repositoryName, cloneUrl,
    ...(defaultBranch ? { defaultBranch } : {}),
  };
}

function credentialFreeRepository(repo) {
  return {
    provider_id: repo.providerId,
    provider_host: repo.providerHost,
    provider_scope: repo.providerScope ?? "",
    provider_repository_id: repo.repositoryId,
    owner_or_project: repo.ownerOrProject,
    name: repo.repositoryName,
    clone_url: repo.cloneUrl,
    default_branch: repo.defaultBranch ?? "",
  };
}

// Gitea icon: a stylized tea cup (simplified).
const GITEA_ICON_PATH = "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.42 0-8-3.58-8-8s3.58-8 8-8 8 3.58 8 8-3.58 8-8 8z";

function giteaIcon(h, size) {
  return h("svg", {
    xmlns: "http://www.w3.org/2000/svg", width: size || 16, height: size || 16,
    viewBox: "0 0 24 24", fill: "none", stroke: "currentColor",
    strokeWidth: 2, strokeLinecap: "round", strokeLinejoin: "round", "aria-hidden": "true",
  }, h("path", { d: GITEA_ICON_PATH }));
}

function parseGiteaPullRequestReference(reference) {
  const trimmed = reference.trim();
  if (!trimmed) return null;
  try {
    const url = new URL(trimmed);
    const parts = url.pathname.split("/").filter(Boolean);
    if (parts.length >= 4 && parts[2] === "pulls") return `${parts[0]}/${parts[1]}#${parts[3]}`;
  } catch { /* not a URL */ }
  if (/^[^/]+\/[^#]+#\d+$/.test(trimmed)) return trimmed;
  return null;
}

window.registerKandevPlugin("kandev-plugin-gitea", {
  initialize(registry, host) {
    const PROVIDER_ID = "gitea";
    const LABEL = "Gitea";
    const NOUN = "pull request";

    const reviewStore = createSnapshotStore();
    const associationStore = createSnapshotStore();
    const overlays = new Set();

    async function refreshReviews(taskId, signal, workspaceId) {
      const token = reviewStore.beginRefresh(taskId);
      signal.throwIfAborted();
      const response = await host.api.invokeAction("change_requests.get", { ...(workspaceId ? { workspaceId } : {}), taskId }, { signal });
      signal.throwIfAborted();
      reviewStore.commit(taskId, token, (response.reviews ?? []).flatMap((r) => {
        const n = normalizeReview(PROVIDER_ID, r);
        return n ? [n] : [];
      }));
    }

    async function refreshAssociations(workspaceId, signal) {
      const token = associationStore.beginRefresh(workspaceId);
      signal.throwIfAborted();
      const response = await host.api.invokeAction("change_requests.associations", { workspaceId }, { signal });
      signal.throwIfAborted();
      associationStore.commit(workspaceId, token, (response.associations ?? []).flatMap((a) => {
        const n = normalizeAssociation(PROVIDER_ID, a);
        return n ? [n] : [];
      }));
    }

    async function refreshAfterMutation(workspaceId, taskId, signal) {
      try { await Promise.all([refreshReviews(taskId, signal, workspaceId), refreshAssociations(workspaceId, signal)]); }
      catch { /* mutation already succeeded */ }
    }

    registry.registerRepositoryProvider({
      id: PROVIDER_ID,
      label: LABEL,
      icon: { render: (h) => giteaIcon(h) },
      supportsDraft: true,

      async listRepositories({ workspaceId, query = "", cursor = "", limit = 100, signal }) {
        signal.throwIfAborted();
        const response = await host.api.invokeAction("repositories.list", { workspaceId, body: { query, cursor, limit } }, { signal });
        signal.throwIfAborted();
        return {
          repositories: (response.repositories ?? []).flatMap((r) => { const n = normalizeRepository(PROVIDER_ID, r); return n ? [n] : []; }),
          nextCursor: text(response.next_cursor) || undefined,
        };
      },

      async listBranches({ workspaceId, repository, signal }) {
        signal.throwIfAborted();
        const response = await host.api.invokeAction("repositories.branches", { workspaceId, body: { repository: credentialFreeRepository(repository) } }, { signal });
        signal.throwIfAborted();
        return (response.branches ?? []).flatMap((b) => { const name = text(record(b).name); return name ? [{ name }] : []; });
      },

      async inspectURL({ workspaceId, url, signal }) {
        signal.throwIfAborted();
        const response = await host.api.invokeAction("repositories.inspect", { workspaceId, body: { url } }, { signal });
        signal.throwIfAborted();
        return normalizeRepository(PROVIDER_ID, response.repository);
      },

      async createChangeRequest({ workspaceId, taskId, sessionId, repositoryId, title, body, baseBranch, draft, signal }) {
        signal.throwIfAborted();
        const response = await host.api.invokeAction("change_requests.create", {
          workspaceId, taskId, sessionId, repositoryId,
          body: { title, description: body, destination: baseBranch ?? "", draft },
        }, { signal });
        const url = text(response.url);
        if (!url) throw new Error("Gitea plugin: create response did not include a URL");
        await refreshAfterMutation(workspaceId, taskId, signal);
        const output = text(response.output);
        const associationError = text(response.association_error);
        return {
          url, provider: PROVIDER_ID,
          ...(output ? { output } : {}),
          ...(typeof response.linked === "boolean" ? { linked: response.linked } : {}),
          ...(associationError ? { associationError } : {}),
        };
      },
    });

    registry.registerTaskAction({
      id: `${PROVIDER_ID}-link-pull-request`,
      label: `${LABEL} ${NOUN}`,
      icon: { render: (h) => giteaIcon(h) },
      placement: "link",
      singleTaskOnly: true,
      async run(context) {
        const dialog = host.openTaskLinkDialog({
          title: `Link ${LABEL} ${NOUN}`,
          description: `Enter a ${LABEL} ${NOUN} URL or reference (owner/repo#number).`,
          inputLabel: "Pull request",
          emptyError: `Enter a valid ${LABEL} ${NOUN} reference.`,
          failureMessage: `Failed to link ${LABEL} ${NOUN}.`,
          successMessage: `${LABEL} ${NOUN} linked`,
          inputTestId: `${PROVIDER_ID}-review-reference`,
          errorTestId: `${PROVIDER_ID}-review-reference-error`,
          submitTestId: `${PROVIDER_ID}-review-reference-submit`,
          async onSubmit(reference, signal) {
            signal.throwIfAborted();
            const parsed = parseGiteaPullRequestReference(reference);
            if (!parsed) throw new Error(`Enter a valid ${LABEL} ${NOUN} reference.`);
            await host.api.invokeAction("change_requests.link", {
              workspaceId: context.workspaceId, taskId: context.taskId, body: { reference: parsed },
            }, { signal });
            await refreshAfterMutation(context.workspaceId, context.taskId, signal);
          },
        });
        overlays.add(dialog);
      },
    });

    registry.registerReviewProvider({
      id: PROVIDER_ID,
      label: LABEL,
      icon: { render: (h) => giteaIcon(h) },
      changeRequestNoun: NOUN,
      order: 100,
      getSnapshot: (taskId) => reviewStore.get(taskId),
      subscribe: (taskId, listener) => reviewStore.subscribe(taskId, listener),
      refresh: (taskId, signal) => refreshReviews(taskId, signal),
      getAssociationSnapshot: (workspaceId) => associationStore.get(workspaceId),
      subscribeAssociations: (workspaceId, listener) => associationStore.subscribe(workspaceId, listener),
      refreshAssociations,
      async unlink({ workspaceId, taskId, connectionScope, repositoryId, changeRequestNumber, signal }) {
        const number = typeof changeRequestNumber === "string" && /^\d+$/.test(changeRequestNumber)
          ? positiveInteger(Number(changeRequestNumber)) : positiveInteger(changeRequestNumber);
        if (!connectionScope.trim() || !repositoryId.trim() || number === undefined)
          throw new Error("Gitea plugin: cannot unlink an incomplete pull request identity");
        signal.throwIfAborted();
        await host.api.invokeAction("change_requests.unlink", {
          workspaceId, taskId, body: { connection_scope: connectionScope, repository_id: repositoryId, number },
        }, { signal });
      },
      ReviewPanel: (props) => {
        const review = reviewStore.get(props.taskId).find((c) =>
          c.reviewKey === props.reviewKey && c.connectionScope === props.connectionScope &&
          c.repositoryId === props.repositoryId && String(c.changeRequestNumber) === String(props.changeRequestNumber)
        );
        return host.jsx(host.ui.ChangeRequestDetail, {
          detail: review ?? null,
          presentation: props.presentation,
          loading: false,
          error: null,
        });
      },
    });

    this._overlays = overlays;
    this._reviewStore = reviewStore;
    this._associationStore = associationStore;
  },

  destroy() {
    if (this._overlays) { this._overlays.forEach((o) => o.close()); this._overlays.clear(); }
    if (this._reviewStore) this._reviewStore.clear();
    if (this._associationStore) this._associationStore.clear();
  },
});
