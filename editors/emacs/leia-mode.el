;;; leia-mode.el --- Major mode for Leia scripts -*- lexical-binding: t; -*-

;; Copyright (C) 2026 Never Labs

;; Author: Never Labs
;; Version: 0.1.0
;; Keywords: languages
;; Package-Requires: ((emacs "26.1"))
;; URL: https://github.com/never-labs/leia

;;; Commentary:

;; Basic editing support for Leia source files and leia.mod module files.
;; The mode intentionally has no external dependencies; commands call the
;; `leia' CLI installed on PATH.

;;; Code:

(require 'compile)
(require 'rx)

(defvar eglot-server-programs)

(defgroup leia nil
  "Editing support for the Leia language."
  :group 'languages
  :prefix "leia-")

(defcustom leia-command "leia"
  "Command used to invoke the Leia CLI."
  :type 'string
  :group 'leia)

(defcustom leia-lsp-command "leia-lsp"
  "Command used to invoke the Leia language server."
  :type 'string
  :group 'leia)

(defcustom leia-indent-offset 4
  "Number of spaces used for each Leia indentation level."
  :type 'integer
  :safe #'integerp
  :group 'leia)

(defconst leia--keywords
  '("break" "continue" "defer" "else" "elseif" "for" "go" "goto" "if"
    "in" "range" "return"))

(defconst leia--declarations
  '("chan" "const" "func" "var"))

(defconst leia--constants
  '("false" "nil" "true"))

(defconst leia--builtins
  '("assert" "cap" "close" "collectgarbage" "dofile" "error" "getmetatable"
    "ipairs" "len" "load" "loadfile" "loadstring" "next" "pairs" "pcall"
    "print" "rawequal" "rawget" "rawlen" "rawset" "require" "select"
    "setmetatable" "spread" "tonumber" "tostring" "type" "unpack" "xpcall"))

(defconst leia--modules
  '("array" "base64" "binary" "bit32" "bits" "bytes" "chat" "color"
    "compress" "container" "context" "control" "crypto" "csv" "data" "db"
    "debug" "encoding" "fs" "hash" "history" "http" "io" "json" "linalg"
    "llm" "log" "loop" "math" "matrix" "msg" "net" "os" "ode" "path"
    "process" "rand" "regexp" "script" "serve" "soa" "sort" "stats"
    "string" "sync" "table" "testkit" "time" "url" "utf8" "uuid" "vec"))

(defconst leia--primitive-types
  '("bool" "f32" "f64" "i32" "i64"))

(defconst leia--number-regexp
  (let* ((decimal "[0-9]+\\(?:_[0-9]+\\)*")
         (exponent (concat "[eE][+-]?" decimal)))
    (concat
     "\\_<\\(?:"
     "0[xX][0-9A-Fa-f]+\\(?:_[0-9A-Fa-f]+\\)*"
     "\\|0[bB][01]+\\(?:_[01]+\\)*"
     "\\|0[oO][0-7]+\\(?:_[0-7]+\\)*"
     "\\|" decimal "\\." decimal "\\(?:" exponent "\\)?"
     "\\|" decimal exponent
     "\\|" decimal
     "\\)\\_>"))
  "Regexp matching portable Leia numeric literals.")

(defconst leia--dialect-tag-regexp
  (rx symbol-start (group (+ (or alnum "_"))) symbol-end
      (? "!") (* (any " \t\n")) (or "`" "{")))

(defun leia--code-position-p (position)
  "Return non-nil when POSITION is outside a string or comment."
  (not (nth 8 (syntax-ppss position))))

(defun leia--dialect-block-context-p (position)
  "Return non-nil when a dialect block tag may begin at POSITION."
  (save-excursion
    (goto-char position)
    (skip-chars-backward " \t")
    (or (bolp)
        (memq (char-before) '(?= ?: ?\( ?\[ ?\{ ?, ?\; ?+ ?- ?* ?/ ?% ?& ?| ?^))
        (let ((end (point)))
          (skip-syntax-backward "w_")
          (equal (buffer-substring-no-properties (point) end) "return")))))

(defun leia--match-dialect-tag (limit)
  "Find the next tagged dialect form before LIMIT."
  (let (found)
    (while (and (not found) (re-search-forward leia--dialect-tag-regexp limit t))
      (let ((match (match-data))
            (resume (point))
            (tag-start (match-beginning 1))
            (tag (match-string-no-properties 1))
            (delimiter (char-before (match-end 0))))
        (when (and (leia--code-position-p tag-start)
                   (leia--dialect-block-context-p tag-start)
                   (or (eq delimiter ?`)
                       (not (and (equal tag "select")
                                 (save-excursion
                                   (goto-char tag-start)
                                   (= (progn (back-to-indentation) (point))
                                      tag-start))))))
          (set-match-data match)
          (setq found t))
        (goto-char resume)))
    found))

(defun leia--match-contextual-keyword (limit)
  "Find the next contextual keyword use before LIMIT."
  (let ((regexp (rx line-start (* (any " \t"))
                    (group (or "evaluate" "import" "select" "case" "default"))
                    symbol-end))
        found)
    (while (and (not found) (re-search-forward regexp limit t))
      (let ((match (match-data))
            (resume (point))
            (word (match-string-no-properties 1))
            valid)
        (save-excursion
          (goto-char (match-end 1))
          (let ((spaces (skip-chars-forward " \t")))
            (setq valid
                  (pcase word
                    ((or "evaluate" "import")
                     (and (> spaces 0) (looking-at-p "[\"'`(]")))
                    ("select" (eq (char-after) ?\{))
                    ("case" (and (> spaces 0)
                                 (not (memq (char-after) '(?= ?:)))))
                    ("default" (eq (char-after) ?:))))))
        (when (and valid (leia--code-position-p (match-beginning 1)))
          (set-match-data match)
          (setq found t))
        (goto-char resume)))
    found))

(defconst leia-font-lock-keywords
  `((,(rx line-start (* (any " \t")) "//leia:"
          (+ (or alnum "_" "-")) (* nonl) line-end)
     0 font-lock-preprocessor-face t)
    (,(rx "```" (*? (or nonl "\n")) "```")
     0 font-lock-string-face t)
    (,(rx (group (literal "$") (? "!"))
          (* (any " \t\n")) "`")
     1 font-lock-preprocessor-face)
    (leia--match-dialect-tag 1 font-lock-preprocessor-face)
    (leia--match-contextual-keyword 1 font-lock-keyword-face)
    (,leia--number-regexp . font-lock-constant-face)
    (,(regexp-opt leia--keywords 'symbols) . font-lock-keyword-face)
    (,(regexp-opt leia--declarations 'symbols) . font-lock-type-face)
    (,(regexp-opt leia--constants 'symbols) . font-lock-constant-face)
    (,(regexp-opt leia--primitive-types 'symbols) . font-lock-type-face)
    (,(regexp-opt leia--builtins 'symbols) . font-lock-builtin-face)
    (,(regexp-opt leia--modules 'symbols) . font-lock-constant-face)
    (,(rx symbol-start (group (+ (or word "_"))) (* space) "(")
     1 font-lock-function-name-face)))

(defconst leia-mod-font-lock-keywords
  `((,(rx line-start (* space)
          (group (or "module" "leia" "require" "replace" "collection"
                     "capability" "cap" "go"))
          symbol-end)
     1 font-lock-keyword-face)
    ("=>" . font-lock-operator-face)
    (,(rx symbol-start "v" (+ (or alnum "." "_" "+" "-")) symbol-end)
     . font-lock-constant-face)
    (,(rx (or "github.com" "go:" "./" "../" "/")
          (+ (or alnum "." "_" "~" ":" "/" "@" "+" "-")))
     . font-lock-string-face)))

(defvar leia-mode-syntax-table
  (let ((table (make-syntax-table)))
    (modify-syntax-entry ?_ "w" table)
    (modify-syntax-entry ?/ ". 124b" table)
    (modify-syntax-entry ?* ". 23" table)
    (modify-syntax-entry ?\n "> b" table)
    (modify-syntax-entry ?\" "\"" table)
    (modify-syntax-entry ?' "\"" table)
    (modify-syntax-entry ?` "\"" table)
    table)
  "Syntax table for `leia-mode'.")

(defun leia--syntax-propertize (start end)
  "Apply fenced raw string syntax properties between START and END."
  (save-excursion
    (goto-char start)
    (while (re-search-forward "```" end t)
      (let ((open (match-beginning 0)))
        (if (nth 8 (parse-partial-sexp (point-min) open))
            (goto-char (match-end 0))
          (let* ((body-start (match-end 0))
                 (close (and (search-forward "```" end t)
                             (- (point) 3)))
                 (raw-end (if close (+ close 3) end)))
            (put-text-property open raw-end
                               'syntax-table (string-to-syntax "."))
            (put-text-property open (1+ open)
                               'syntax-table (string-to-syntax "\""))
            (when close
              (put-text-property (+ close 2) (+ close 3)
                                 'syntax-table (string-to-syntax "\"")))
            (goto-char (if close raw-end body-start))))))))

(defvar leia-mod-mode-syntax-table
  (let ((table (make-syntax-table)))
    (modify-syntax-entry ?_ "w" table)
    (modify-syntax-entry ?/ ". 12b" table)
    (modify-syntax-entry ?\n "> b" table)
    (modify-syntax-entry ?\" "\"" table)
    table)
  "Syntax table for `leia-mod-mode'.")

(defun leia--line-starts-with-closing-delimiter-p ()
  "Return non-nil if the current line starts with a closing delimiter."
  (save-excursion
    (back-to-indentation)
    (looking-at-p "[]})]")))

(defun leia--previous-code-line-indentation ()
  "Return indentation of the previous non-empty, non-comment-only line."
  (save-excursion
    (let ((indent 0)
          (found nil))
      (while (and (not found) (zerop (forward-line -1)))
        (back-to-indentation)
        (unless (or (eolp) (looking-at-p "//"))
          (setq indent (current-indentation))
          (setq found t)))
      indent)))

(defun leia--previous-code-line-opens-block-p ()
  "Return non-nil if the previous code line appears to open a block."
  (save-excursion
    (let ((opens nil)
          (found nil))
      (while (and (not found) (zerop (forward-line -1)))
        (back-to-indentation)
        (unless (or (eolp) (looking-at-p "//"))
          (end-of-line)
          (skip-chars-backward " \t")
          (setq opens (and (> (point) (line-beginning-position))
                           (memq (char-before) '(?\{ ?\( ?\[))))
          (setq found t)))
      opens)))

(defun leia-indent-line ()
  "Indent current Leia line using delimiter-oriented rules."
  (interactive)
  (let* ((base (leia--previous-code-line-indentation))
         (indent (+ base (if (leia--previous-code-line-opens-block-p)
                             leia-indent-offset
                           0))))
    (when (leia--line-starts-with-closing-delimiter-p)
      (setq indent (max 0 (- indent leia-indent-offset))))
    (indent-line-to indent)))

(defun leia--buffer-file-name ()
  "Return current buffer file name or signal a user-facing error."
  (or buffer-file-name
      (user-error "Current buffer is not visiting a file")))

(defun leia--compile (command)
  "Run COMMAND through `compile'."
  (compile command))

;;;###autoload
(defun leia-eglot-setup ()
  "Register `leia-lsp' with Eglot for `leia-mode'.
This function does not require Eglot at load time; call it from init after
Eglot is available."
  (interactive)
  (require 'eglot)
  (add-to-list 'eglot-server-programs
               `(leia-mode . (,leia-lsp-command))))

;;;###autoload
(defun leia-run-current-file (&optional vm)
  "Run the current Leia file with the Leia CLI.
With prefix argument VM, run through the bytecode VM without JIT."
  (interactive "P")
  (save-buffer)
  (let ((file (shell-quote-argument (leia--buffer-file-name))))
    (leia--compile
     (format "%s run %s %s"
             (shell-quote-argument leia-command)
             (if vm "-vm" "")
             file))))

;;;###autoload
(defun leia-format-current-file ()
  "Format the current Leia file in place with `leia fmt -write'."
  (interactive)
  (save-buffer)
  (let ((file (shell-quote-argument (leia--buffer-file-name))))
    (leia--compile
     (format "%s fmt -write %s"
             (shell-quote-argument leia-command)
             file))))

;;;###autoload
(defun leia-check-format-current-file ()
  "Check whether the current Leia file is formatted."
  (interactive)
  (save-buffer)
  (let ((file (shell-quote-argument (leia--buffer-file-name))))
    (leia--compile
     (format "%s fmt -check %s"
             (shell-quote-argument leia-command)
             file))))

;;;###autoload
(defun leia-test (&optional path)
  "Run `leia test' for PATH, defaulting to the current project root."
  (interactive
   (list (read-directory-name "Leia test path: "
                              (or (locate-dominating-file default-directory "leia.mod")
                                  default-directory)
                              nil t)))
  (leia--compile
   (format "%s test %s"
           (shell-quote-argument leia-command)
           (shell-quote-argument (or path default-directory)))))

(defvar leia-mode-map
  (let ((map (make-sparse-keymap)))
    (define-key map (kbd "C-c C-r") #'leia-run-current-file)
    (define-key map (kbd "C-c C-f") #'leia-format-current-file)
    (define-key map (kbd "C-c C-c") #'leia-check-format-current-file)
    (define-key map (kbd "C-c C-t") #'leia-test)
    map)
  "Keymap for `leia-mode'.")

;;;###autoload
(define-derived-mode leia-mode prog-mode "Leia"
  "Major mode for editing Leia source files."
  :syntax-table leia-mode-syntax-table
  (setq-local font-lock-defaults '(leia-font-lock-keywords))
  (setq-local syntax-propertize-function #'leia--syntax-propertize)
  (setq-local comment-start "// ")
  (setq-local comment-end "")
  (setq-local comment-start-skip "\\(?://+\\|/\\*+\\)\\s *")
  (setq-local indent-line-function #'leia-indent-line)
  (setq-local tab-width leia-indent-offset))

;;;###autoload
(define-derived-mode leia-mod-mode prog-mode "LeiaMod"
  "Major mode for editing leia.mod module files."
  :syntax-table leia-mod-mode-syntax-table
  (setq-local font-lock-defaults '(leia-mod-font-lock-keywords))
  (setq-local comment-start "// ")
  (setq-local comment-end "")
  (setq-local indent-line-function #'leia-indent-line)
  (setq-local tab-width leia-indent-offset))

;;;###autoload
(add-to-list 'auto-mode-alist '("\\.leia\\'" . leia-mode))

;;;###autoload
(add-to-list 'auto-mode-alist '("/leia\\.mod\\'" . leia-mod-mode))

(provide 'leia-mode)

;;; leia-mode.el ends here
