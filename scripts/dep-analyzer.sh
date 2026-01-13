#!/usr/bin/env bash
set -euo pipefail

# dep-analyzer.sh — analyze and slice DOT dependency graphs
# Preserves existing semantic colors from os-image-composer

usage() {
  cat <<'EOF'
dep-analyzer.sh — analyze and slice DOT dependency graphs

Usage:
  ./dep-analyzer.sh -i deps.dot -r vim -d 2              # Forward deps of vim, depth 2
  ./dep-analyzer.sh -i deps.dot -r vim -d 3 --reverse    # Who depends on vim, depth 3
  ./dep-analyzer.sh -i deps.dot -r vim -d 2 -t svg       # Render to SVG

Options:
  -i, --input FILE      Input DOT file (required)
  -r, --root NAME       Root package name (required for slicing)
  -d, --depth N         Max depth, default: 2
  -o, --output FILE     Output file (auto-generated if omitted)
  -t, --type TYPE       Output: dot|svg|png|pdf (default: dot)
      --reverse         Reverse direction (who depends on root)
      --highlight-root  Add thick red border to root node
      --list-roots      List all root packages (no incoming edges)
      --list-leaves     List all leaf packages (no outgoing edges)
      --stats           Show graph statistics
  -h, --help            Show this help

Notes:
  - Preserves semantic colors from os-image-composer DOT output:
    * Yellow (#fff4d6) = Essential packages
    * Green (#d4efdf) = System packages (user-specified)
    * Blue (#d6eaf8) = Kernel packages
    * Orange (#fdebd0) = Bootloader packages
  - "depth" means shortest-path hops from root following edge direction
  - Reverse mode is useful for "who depends on X" analysis
EOF
}

INPUT="" ROOT="" DEPTH=2 OUT="" TYPE="dot"
REVERSE="false" HIGHLIGHT="false"
LIST_ROOTS="false" LIST_LEAVES="false" SHOW_STATS="false"
DEPTH_SET="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    -i|--input) INPUT="$2"; shift 2;;
    -r|--root) ROOT="$2"; shift 2;;
    -d|--depth) DEPTH="$2"; DEPTH_SET="true"; shift 2;;
    -o|--output) OUT="$2"; shift 2;;
    -t|--type) TYPE="$2"; shift 2;;
    --reverse) REVERSE="true"; shift;;
    --highlight-root) HIGHLIGHT="true"; shift;;
    --list-roots) LIST_ROOTS="true"; shift;;
    --list-leaves) LIST_LEAVES="true"; shift;;
    --stats) SHOW_STATS="true"; shift;;
    -h|--help) usage; exit 0;;
    *) echo "Unknown: $1"; usage; exit 2;;
  esac
done

# Helper function for graphviz installation error
graphviz_not_found() {
  echo "Error: $1 not found (part of graphviz package)"
  echo ""
  echo "Install graphviz using your package manager:"
  echo "  Ubuntu/Debian:  sudo apt install graphviz"
  echo "  Fedora/RHEL:    sudo dnf install graphviz"
  echo "  openSUSE:       sudo zypper install graphviz"
  exit 1
}

[[ -z "${INPUT}" ]] && { echo "Error: --input required"; usage; exit 2; }
[[ ! -f "${INPUT}" ]] && { echo "Error: File not found: ${INPUT}"; exit 1; }
command -v gvpr >/dev/null || graphviz_not_found "gvpr"

# Handle list/stats modes that don't require --root
# Note: use gvpr without -c to avoid outputting the graph
if [[ "${LIST_ROOTS}" == "true" ]]; then
  echo "Root packages (no incoming edges):"
  gvpr 'N { int c = 0; edge_t e; for (e = fstin($); e; e = nxtin(e)) c++; if (c == 0) print($.name); }' "${INPUT}" | sort -u
  exit 0
fi

if [[ "${LIST_LEAVES}" == "true" ]]; then
  echo "Leaf packages (no outgoing edges):"
  gvpr 'N { int c = 0; edge_t e; for (e = fstout($); e; e = nxtout(e)) c++; if (c == 0) print($.name); }' "${INPUT}" | sort -u
  exit 0
fi

if [[ "${SHOW_STATS}" == "true" ]]; then
  # Count nodes by iterating
  nodes=$(gvpr 'BEG_G { int n = 0; node_t v; for (v = fstnode($G); v; v = nxtnode(v)) n++; print(n); }' "${INPUT}")
  # Count edges - iterate over all nodes and their out-edges
  edges=$(gvpr 'BEG_G { int n = 0; node_t v; edge_t e; for (v = fstnode($G); v; v = nxtnode(v)) for (e = fstout(v); e; e = nxtout(e)) n++; print(n); }' "${INPUT}")
  roots=$(gvpr 'N { int c = 0; edge_t e; for (e = fstin($); e; e = nxtin(e)) c++; if (c == 0) print($.name); }' "${INPUT}" | wc -l)
  leaves=$(gvpr 'N { int c = 0; edge_t e; for (e = fstout($); e; e = nxtout(e)) c++; if (c == 0) print($.name); }' "${INPUT}" | wc -l)
  echo "Graph Statistics for: ${INPUT}"
  echo "  Total packages: ${nodes}"
  echo "  Total dependencies: ${edges}"
  echo "  Root packages (no deps on them): ${roots}"
  echo "  Leaf packages (no deps): ${leaves}"
  exit 0
fi

# For slicing mode, --root is required
[[ -z "${ROOT}" ]] && { echo "Error: --root required for slicing"; usage; exit 2; }
! [[ "${DEPTH}" =~ ^[0-9]+$ ]] && { echo "Error: depth must be an integer >= 0"; exit 2; }

stem="$(basename "${INPUT}" .dot)"
# Build filename with optional suffixes for depth, reverse
if [[ -z "${OUT}" ]]; then
  OUT="${stem}_${ROOT}"
  [[ "${DEPTH_SET}" == "true" ]] && OUT="${OUT}_d${DEPTH}"
  [[ "${REVERSE}" == "true" ]] && OUT="${OUT}_reverse"
  OUT="${OUT}.${TYPE}"
fi

tmp_dot="$(mktemp)"
trap 'rm -f "$tmp_dot"' EXIT

# Convert bash booleans to gvpr integers
REV_INT=0; [[ "${REVERSE}" == "true" ]] && REV_INT=1
HL_INT=0; [[ "${HIGHLIGHT}" == "true" ]] && HL_INT=1

# gvpr BFS slicer - preserves original node colors
gvpr -c '
BEGIN {
  string rootName = "'"${ROOT}"'";
  int maxd = '"${DEPTH}"';
  int rev = '"${REV_INT}"';
  int hl = '"${HL_INT}"';
  int dist[node_t];
  node_t Q[int];
  int qh = 0;
  int qt = 0;
}

BEG_G {
  node_t n;
  for (n = fstnode($G); n; n = nxtnode(n)) {
    dist[n] = -1;
  }

  node_t startNode = isNode($G, rootName);
  if (startNode == NULL) {
    printf(2, "Error: node \"%s\" not found in graph\n", rootName);
    exit(1);
  }

  dist[startNode] = 0;
  Q[qt] = startNode;
  qt = qt + 1;

  node_t v;
  node_t w;
  edge_t e;

  while (qh < qt) {
    v = Q[qh];
    qh = qh + 1;
    if (dist[v] >= maxd) continue;

    if (rev == 1) {
      for (e = fstin(v); e; e = nxtin(e)) {
        w = e.tail;
        if (dist[w] < 0) {
          dist[w] = dist[v] + 1;
          Q[qt] = w;
          qt = qt + 1;
        }
      }
    } else {
      for (e = fstout(v); e; e = nxtout(e)) {
        w = e.head;
        if (dist[w] < 0) {
          dist[w] = dist[v] + 1;
          Q[qt] = w;
          qt = qt + 1;
        }
      }
    }
  }
}

E {
  if (dist[$.tail] < 0 || dist[$.head] < 0) {
    delete($G, $);
  }
}

N {
  if (dist[$] < 0) {
    delete($G, $);
  } else if (hl == 1 && dist[$] == 0) {
    $.penwidth = "3";
    $.style = "filled,bold";
  }
}
' "${INPUT}" > "${tmp_dot}"

# Check if output has the root node (for depth 0, there may be no edges)
if ! grep -qE "\"?${ROOT}\"?\s*\[" "${tmp_dot}" && ! grep -qE "^\s*${ROOT}\s*\[" "${tmp_dot}"; then
  echo "Warning: Root node '${ROOT}' not found in output."
  echo "Tip: Use 'grep \"${ROOT}\" ${INPUT}' to verify the node name."
fi

if [[ "${TYPE}" == "dot" ]]; then
  cp "${tmp_dot}" "${OUT}"
  echo "Wrote: ${OUT}"
  exit 0
fi

command -v dot >/dev/null || graphviz_not_found "dot"

case "${TYPE}" in
  svg|png|pdf)
    dot -T"${TYPE}" "${tmp_dot}" -o "${OUT}"
    echo "Rendered: ${OUT}"
    ;;
  *)
    echo "Error: unsupported type '${TYPE}'. Use dot|svg|png|pdf."
    exit 2
    ;;
esac
