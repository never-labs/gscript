;;; leia-mode-test.el --- Tests for leia-mode -*- lexical-binding: t; -*-

(require 'ert)
(require 'leia-mode)

(defmacro leia-test--with-fontified-buffer (source &rest body)
  "Fontify SOURCE in `leia-mode' and evaluate BODY."
  (declare (indent 1) (debug t))
  `(with-temp-buffer
     (insert ,source)
     (leia-mode)
     (font-lock-ensure)
     ,@body))

(defun leia-test--face-at (text)
  "Return the face at the first occurrence of TEXT."
  (save-excursion
    (goto-char (point-min))
    (search-forward text)
    (get-text-property (- (point) (length text)) 'face)))

(ert-deftest leia-font-lock-reserved-keywords ()
  (leia-test--with-fontified-buffer
      "if ready { return true }\nswitch := 1\nfallthrough := 2\nselect := 3\n"
    (should (eq (leia-test--face-at "if") 'font-lock-keyword-face))
    (should (eq (leia-test--face-at "return") 'font-lock-keyword-face))
    (dolist (word '("switch" "fallthrough" "select"))
      (should-not (eq (leia-test--face-at word) 'font-lock-keyword-face)))))

(ert-deftest leia-font-lock-strings ()
  (leia-test--with-fontified-buffer
      (concat "single := 'hello ${name}'\n"
              "double := \"hello\"\n"
              "short := `raw`\n"
              "fenced := ```one ` two `` three\n+four```\n")
    (dolist (text '("hello ${name}" "hello" "raw"
                    "one ` two `` three" "four"))
      (should (eq (leia-test--face-at text) 'font-lock-string-face)))
    (goto-char (point-min))
    (search-forward "four")
    (should (nth 3 (syntax-ppss (point))))))

(ert-deftest leia-font-lock-numbers ()
  (leia-test--with-fontified-buffer
      "a := 1_000\nb := 0xff\nc := 0b10_01\nd := 0o755\ne := 1.25e-2\nf := 2E+4\ng := 1..2\nbad := 1__2\n"
    (dolist (number '("1_000" "0xff" "0b10_01" "0o755" "1.25e-2" "2E+4"))
      (should (eq (leia-test--face-at number) 'font-lock-constant-face)))
    (should-not (eq (leia-test--face-at "1__2") 'font-lock-constant-face))))

(ert-deftest leia-font-lock-contextual-words ()
  (leia-test--with-fontified-buffer
      (concat "agent := 1\ntool := 2\nmodel := 3\nturn := 4\n"
              "evaluate := 5\n"
              "evaluate 'case' { assert(true) }\n"
              "import 'example/module'\n"
              "select {\ncase <-events:\ndefault:\n}\n")
    (dolist (word '("agent" "tool" "model" "turn"))
      (should-not (eq (leia-test--face-at word) 'font-lock-keyword-face)))
    (goto-char (point-min))
    (search-forward "evaluate")
    (should-not (eq (get-text-property (match-beginning 0) 'face)
                    'font-lock-keyword-face))
    (search-forward "evaluate")
    (should (eq (get-text-property (match-beginning 0) 'face)
                'font-lock-keyword-face))
    (dolist (word '("import" "select" "case" "default"))
      (goto-char (point-min))
      (re-search-forward (concat "^[ \t]*\\(" word "\\)\\_>"))
      (should (eq (get-text-property (match-beginning 1) 'face)
                  'font-lock-keyword-face)))))

(ert-deftest leia-font-lock-dialect-tags-without-control-flow-false-positive ()
  (leia-test--with-fontified-buffer
      (concat "if ready {\n    return true\n}\n"
              "result := prompt { role: 'system' }\n"
              "text := prompt```hello```\n"
              "shell := $!`exit 1`\n")
    (should-not (eq (leia-test--face-at "ready")
                    'font-lock-preprocessor-face))
    (dolist (tag '("prompt" "$!`"))
      (should (eq (leia-test--face-at tag)
                  'font-lock-preprocessor-face)))))

(ert-deftest leia-font-lock-exact-directives ()
  (leia-test--with-fontified-buffer
      "//leia:cap fs.read\n// leia:cap fs.read\n//@leia:test smoke\n"
    (should (eq (leia-test--face-at "//leia:cap")
                'font-lock-preprocessor-face))
    (should-not (eq (leia-test--face-at "// leia:cap")
                    'font-lock-preprocessor-face))
    (should-not (eq (leia-test--face-at "//@leia:test")
                    'font-lock-preprocessor-face))))

(provide 'leia-mode-test)

;;; leia-mode-test.el ends here
