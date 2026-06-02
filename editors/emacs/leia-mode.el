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
  '("break" "case" "continue" "defer" "default" "else" "elseif" "for"
    "go" "goto" "if" "in" "range" "return" "select" "switch"
    "fallthrough"))

(defconst leia--declarations
  '("chan" "const" "func" "import" "tool" "var"))

(defconst leia--contextual-keywords
  '("agent" "budget" "capabilities" "flow" "messages" "models" "react" "turn"))

(defconst leia--constants
  '("false" "nil" "true"))

(defconst leia--builtins
  '("assert" "cap" "close" "delete" "error" "getmetatable" "ipairs" "len"
    "make" "next" "pairs" "pcall" "print" "rawequal" "rawget" "rawlen"
    "rawset" "recv" "require" "select" "send" "setmetatable" "sleep" "spawn"
    "spread" "tonumber" "tostring" "type" "wait" "xpcall"))

(defconst leia--modules
  '("base64" "binary" "bits" "bit32" "bytes" "compress" "csv" "debug" "env"
    "fs" "hash" "history" "http" "json" "llm" "loop" "math" "matrix" "msg"
    "os" "path" "process" "regexp" "soa" "string" "sync" "table" "time"
    "url" "uuid" "vec"))

(defconst leia--primitive-types
  '("bool" "f32" "f64" "i32" "i64"))

(defconst leia-font-lock-keywords
  `((,(rx "//leia:" (+ (or word "_" "." "-"))) . font-lock-preprocessor-face)
    (,(regexp-opt leia--keywords 'symbols) . font-lock-keyword-face)
    (,(regexp-opt leia--declarations 'symbols) . font-lock-type-face)
    (,(regexp-opt leia--contextual-keywords 'symbols) . font-lock-keyword-face)
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
    (modify-syntax-entry ?` "\"" table)
    table)
  "Syntax table for `leia-mode'.")

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
