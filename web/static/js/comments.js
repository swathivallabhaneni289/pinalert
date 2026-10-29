// comments.js is the comments half of the report page (Phase 8 plan 16, PAGE-02,
// D-02, D-12, D-13, D-15, UI-SPEC Surface 7). It is mount agnostic: the page
// builder (report-page.js) gives it an empty mount point and forwards update,
// open and close, and it holds no breakpoint test and no host class. The phone
// and the laptop panel draw the same nodes; placement and the line arrangement
// are CSS (comments.css, phone.css, report-panel.css).
//
// The server decides who sees what: held comments reach only their author,
// names that have not cleared screening arrive as New member, a deleted or
// removed comment arrives as a placeholder. The client never filters and never
// guesses; it shows what it is sent. Every word, name and server sentence
// reaches the page through text nodes (Pinalert.setText, createTextNode), so no
// markup is ever parsed. The one bold part of a comment is the leading
// @username of a reply, made with createElement, and only when it is exactly
// the name the server sent as reply_to_author (review W3). Plan 08-17 adds Like
// (a thumbs up and nothing else, the count and the pressed state written only
// from the server's answer), Delete (the author's, behind the app's confirm
// box) and Flag (everyone else's, through the Flag dialog's comment mode) to
// the meta line after the Reply button. A deleted or removed placeholder has
// none of them.
//
// The list is built once and refreshed in place: a node map by comment id
// means a poll, a post or a vote updates or inserts nodes and never rebuilds
// the list or the box, so typed words, focus, the selection and every open
// thread survive (research Pitfall 6). The box counts code points with
// Array.from, as the server counts runes; the textarea carries no length
// attribute because that counts UTF-16 units and would cut Telugu or emoji
// text short (research Pitfall 7, review W2). The box stops every key event at
// the form so Leaflet's keyboard handling and map.js never see typing
// (research Pitfall 9).
window.PinalertComments = (function () {
  'use strict';

  var COMMENTS_POLL_MS = 30000;
  var COMMENTS_TIMEOUT_MS = 10000;
  var COMMENT_MAX = 280;
  var COUNTER_FROM = 40;
  var COUNTER_NEAR = 10;
  var COUNTER_END = 0;
  var SKELETON_COMMENTS = 3;
  var INPUT_MAX_HEIGHT = 120;
  var STATUS_CREATED = 201;
  var STATUS_CONFLICT = 409;
  var STATUS_GONE = 404;

  var HEADING = 'Comments';
  var REPLY_LABEL = 'Reply';
  var REPORTER_TAG = 'Reporter';
  var VIEW_ONE = 'View 1 reply';
  var HIDE_REPLIES = 'Hide replies';
  var SHOW_MORE = 'Show more comments';
  var PLACEHOLDER = 'Add a comment…';
  var ASK_LABEL = 'Add a comment';
  var POST_LABEL = 'Post';
  var POSTING_LABEL = 'Posting…';
  var REPLYING_TO = 'Replying to ';
  var CANCEL_REPLY = 'Cancel reply';
  var POSTED_STATUS = 'Comment posted.';
  var EMPTY_LINE = 'No comments yet.';
  var CLOSED_LINE = 'Comments are closed';
  var HELD_LINE = 'Waiting for a text check. Only you can see it until then.';
  var ERROR_TITLE = "Couldn't load comments";
  var ERROR_LINE = 'Check your connection and try again.';
  var OFFLINE_LINE = "You're offline, so comments can't load. Try again when you have a signal.";
  var RETRY_LABEL = 'Try again';
  var MORE_ERROR = "Couldn't load more comments. Try again.";
  var SHOW_MORE_REPLIES = 'Show more replies';
  var MORE_REPLIES_ERROR = "Couldn't load more replies. Try again.";
  var OFFLINE_POST = "You're offline, so this comment wasn't posted. Your words are still here. Try again when you have a signal.";
  var POST_ERROR = "Couldn't post your comment. Try again.";
  var DELETED_LINE = 'Comment deleted';
  var REMOVED_LINE = 'Comment removed';
  // P9-C31: a comment of someone you blocked that others answered (D-12).
  var HIDDEN_LINE = 'Comment hidden';
  var KIND_USER = 'user';
  var LIKE_LABEL = 'Like';
  var DELETE_LABEL = 'Delete';
  var DELETE_NAME = 'Delete your comment';
  var FLAG_LABEL = 'Flag';
  var FLAG_NAME = 'Flag this comment';
  var FLAGGED_LABEL = 'Flagged';
  var DELETED_STATUS = 'Comment deleted.';
  var OFFLINE_ACTION = "You're offline, so this wasn't saved. Try again when you have a signal.";
  var ACTION_ERROR = "Couldn't save that. Try again.";
  var STATUS_OK = 200;
  var DELETE_QUESTION = 'Delete this comment?';
  var DELETE_BOX_ID = 'comment-delete-confirm';
  var DELETE_TITLE_ID = 'comment-delete-title';
  var DELETE_CANCEL_ID = 'comment-delete-cancel';
  var DELETE_YES_ID = 'comment-delete-yes';

  var seqId = 0;

  // The Delete box is one dialog in the page (index.html.tmpl), shared by every
  // comments section, so its buttons are wired once and the section that asked
  // is the one in pendingDelete (Phase 8 plan 17, D-14). It is the .logout-confirm
  // box and behaves like it: the title takes focus, no button looks chosen, only
  // its buttons close it.
  var pendingDelete = null;
  var boundDeleteBox = null;

  function likeName(count) {
    if (count === 1) {
      return LIKE_LABEL + ', 1 like';
    }
    return count > 1 ? LIKE_LABEL + ', ' + count + ' likes' : LIKE_LABEL;
  }

  function likesName(count) {
    return count === 1 ? '1 like' : count + ' likes';
  }

  function onDeleteBoxClose() {
    var asked = pendingDelete;
    pendingDelete = null;
    if (asked && !asked.answered && asked.opener && typeof asked.opener.focus === 'function') {
      asked.opener.focus();
    }
  }

  // openDeleteBox returns false when the page has no box (the browser's own
  // confirm then asks, with the box's own title).
  function openDeleteBox(asked) {
    var box = document.getElementById(DELETE_BOX_ID);
    var title = document.getElementById(DELETE_TITLE_ID);
    var cancelButton = document.getElementById(DELETE_CANCEL_ID);
    var yesButton = document.getElementById(DELETE_YES_ID);
    if (!box || !title || !cancelButton || !yesButton || typeof box.showModal !== 'function') {
      return false;
    }
    if (boundDeleteBox !== box) {
      boundDeleteBox = box;
      cancelButton.addEventListener('click', function () {
        box.close();
      });
      yesButton.addEventListener('click', function () {
        var chosen = pendingDelete;
        if (chosen) {
          chosen.answered = true;
        }
        box.close();
        if (chosen) {
          chosen.run();
        }
      });
      box.addEventListener('close', onDeleteBoxClose);
    }
    pendingDelete = asked;
    box.showModal();
    // The question takes the first focus and no button looks chosen, as the
    // Log out box does (owner, 2026-10-22).
    if (typeof title.focus === 'function') {
      title.focus();
    }
    return true;
  }

  function element(tag, className) {
    var el = document.createElement(tag);
    el.className = className;
    return el;
  }

  function writeText(node, words) {
    if (node.textContent !== words) {
      Pinalert.setText(node, words);
    }
  }

  function writeClass(node, className) {
    if (node.className !== className) {
      node.className = className;
    }
  }

  function codePoints(text) {
    return Array.from(text).length;
  }

  function viewReplies(count) {
    return count === 1 ? VIEW_ONE : 'View ' + count + ' replies';
  }

  function leftWords(remaining) {
    return remaining + ' left';
  }

  // create builds the comments section into mount. The nodes are made here,
  // once. The returned object is { el, update, open, close, refresh }.
  function create(report, mount) {
    var reportId = String(report.id);
    var current = report;
    var active = false;
    // none, skeleton, error or rows: what the list area shows.
    var mode = 'none';
    var commentsOpen = report.comments_open !== false;
    var total = typeof report.comment_count === 'number' ? report.comment_count : 0;
    var lastFeedCount = total;
    var more = false;
    var nextAfter = 0;
    var loadedMore = false;
    // The after cursor of the last page Show more read, which the poll reads
    // too once Show more has been used (review fix WR-B04).
    var lastPageAfter = 0;
    // Counts the Like answers. A list answer asked for before the latest Like
    // answer of a comment keeps that comment's like state (review fix WR-B05).
    var answerStamp = 0;
    // Open capped threads whose read tail is read again after the answer
    // being written (review fix round 3, SK2-01).
    var pendingTails = [];
    var loadSeq = 0;
    var inflight = null;
    var pollTimer = null;
    var posting = false;
    var ownName = '';
    var spokenBand = Infinity;
    var replyTarget = null;
    // Comment id (as a string) to its record, for roots and replies alike.
    var nodes = {};
    var rootIds = [];
    var openThreads = {};

    seqId += 1;
    var uid = 'comments-' + seqId;

    var root = element('section', 'comments');
    var heading = element('h2', 'comments__heading');
    heading.id = uid + '-heading';
    heading.setAttribute('tabindex', '-1');
    var headingWord = element('span', 'comments__title');
    Pinalert.setText(headingWord, HEADING);
    var countNode = element('span', 'comments__count');
    countNode.hidden = true;
    heading.appendChild(headingWord);
    heading.appendChild(countNode);

    var list = element('ul', 'comments__list');
    list.setAttribute('aria-labelledby', heading.id);
    list.setAttribute('aria-busy', 'false');
    list.hidden = true;

    var emptyLine = element('p', 'comments__empty');
    Pinalert.setText(emptyLine, EMPTY_LINE);
    emptyLine.hidden = true;

    var moreButton = element('button', 'comments__more');
    moreButton.setAttribute('type', 'button');
    Pinalert.setText(moreButton, SHOW_MORE);
    moreButton.hidden = true;
    var moreError = element('p', 'inline-error comments__more-error');
    Pinalert.setText(moreError, MORE_ERROR);
    moreError.setAttribute('role', 'alert');
    moreError.hidden = true;

    var errorBlock = element('div', 'comments__error');
    errorBlock.hidden = true;
    var errorTitle = element('p', 'comments__error-title');
    var errorLine = element('p', 'comments__error-line');
    var retry = element('button', 'comments__retry');
    retry.setAttribute('type', 'button');
    Pinalert.setText(retry, RETRY_LABEL);
    errorBlock.appendChild(errorTitle);
    errorBlock.appendChild(errorLine);
    errorBlock.appendChild(retry);

    var closedLine = element('p', 'comments__closed');
    Pinalert.setText(closedLine, CLOSED_LINE);
    closedLine.hidden = true;

    // The one box, for comments and replies.
    var form = element('form', 'composer');
    form.setAttribute('novalidate', 'novalidate');
    form.hidden = true;
    var replying = element('div', 'composer__replying');
    replying.hidden = true;
    var replyingText = element('p', 'composer__replying-text');
    var replyingName = element('strong', 'composer__replying-name');
    replyingText.appendChild(document.createTextNode(REPLYING_TO));
    replyingText.appendChild(replyingName);
    var cancel = element('button', 'composer__cancel');
    cancel.setAttribute('type', 'button');
    cancel.setAttribute('aria-label', CANCEL_REPLY);
    replying.appendChild(replyingText);
    replying.appendChild(cancel);
    var row = element('div', 'composer__row');
    var ownDisc = element('span', 'comment__disc comment__disc--own');
    ownDisc.setAttribute('aria-hidden', 'true');
    var field = element('div', 'composer__field');
    var input = element('textarea', 'composer__input');
    input.setAttribute('rows', '1');
    input.setAttribute('aria-label', ASK_LABEL);
    input.setAttribute('placeholder', PLACEHOLDER);
    input.setAttribute('enterkeyhint', 'send');
    input.setAttribute('autocapitalize', 'sentences');
    input.setAttribute('autocomplete', 'off');
    var post = element('button', 'composer__post');
    post.setAttribute('type', 'button');
    post.setAttribute('aria-disabled', 'true');
    Pinalert.setText(post, POST_LABEL);
    field.appendChild(input);
    field.appendChild(post);
    row.appendChild(ownDisc);
    row.appendChild(field);
    var composerError = element('p', 'inline-error composer__error');
    composerError.setAttribute('role', 'alert');
    composerError.hidden = true;
    var counter = element('p', 'composer__counter');
    counter.hidden = true;
    var status = element('p', 'composer__status visually-hidden');
    status.setAttribute('role', 'status');
    form.appendChild(replying);
    form.appendChild(row);
    form.appendChild(composerError);
    form.appendChild(counter);
    form.appendChild(status);

    // A hidden status line of its own for a delete, because the box's own line
    // is out of the page while comments are closed and Delete still works then.
    var listStatus = element('p', 'comments__status visually-hidden');
    listStatus.setAttribute('role', 'status');

    function speakDeleted() {
      Pinalert.setText(listStatus, '');
      Pinalert.setText(listStatus, DELETED_STATUS);
    }

    root.appendChild(heading);
    root.appendChild(listStatus);
    root.appendChild(list);
    root.appendChild(emptyLine);
    root.appendChild(moreButton);
    root.appendChild(moreError);
    root.appendChild(errorBlock);
    root.appendChild(closedLine);
    root.appendChild(form);
    mount.appendChild(root);

    // ---- one comment ----

    function isUser(data) {
      return data.author_kind === KIND_USER && !data.deleted && !data.removed && !data.hidden;
    }

    function discKey(data) {
      if (data.deleted || data.removed || data.hidden) {
        return 'empty';
      }
      if (data.author_kind !== KIND_USER) {
        return 'glyph';
      }
      if (data.is_reporter === true) {
        return 'reporter';
      }
      return 'letter:' + data.author;
    }

    function writeDisc(rec, data) {
      var key = discKey(data) + (rec.isReply ? '|s' : '|l') + '|' + (data.is_reporter === true ? current.category + '|' + (current.severity || '') : '');
      if (rec.discKey === key) {
        return;
      }
      rec.discKey = key;
      var disc = rec.disc;
      while (disc.firstChild) {
        disc.removeChild(disc.firstChild);
      }
      var kind = discKey(data);
      var size = rec.isReply ? ' comment__disc--small' : '';
      if (kind === 'reporter') {
        disc.className = 'comment__disc icon-badge icon-badge--sm ' + Pinalert.severityClass(current) + size;
        disc.appendChild(element('span', Pinalert.iconClass(current.category)));
      } else if (kind === 'glyph') {
        disc.className = 'comment__disc comment__disc--glyph' + size;
      } else if (kind === 'empty') {
        disc.className = 'comment__disc comment__disc--empty' + size;
      } else {
        disc.className = 'comment__disc' + size;
        var first = Array.from(String(data.author || ''))[0] || '';
        Pinalert.setText(disc, first.toUpperCase());
      }
    }

    function makeRecord(data, isReply, rootId) {
      var li = element('li', isReply ? 'comment comment--reply' : 'comment');
      var rowEl = element('div', 'comment__row');
      var disc = element('span', 'comment__disc');
      disc.setAttribute('aria-hidden', 'true');
      var body = element('div', 'comment__body');
      var head = element('p', 'comment__head');
      var author = element('span', 'comment__author');
      var tag = element('span', 'comment__tag');
      Pinalert.setText(tag, REPORTER_TAG);
      tag.hidden = true;
      var timeHead = element('time', 'comment__time comment__time--head');
      head.appendChild(author);
      head.appendChild(tag);
      head.appendChild(timeHead);
      var words = element('p', 'comment__words');
      var gone = element('p', 'comment__gone');
      gone.hidden = true;
      // The meta line: the time on the laptop arrangement (the phone has it on
      // the head line), then the controls. Plan 08-17 adds Like, Delete and
      // Flag after the Reply button.
      var meta = element('div', 'comment__meta');
      var timeMeta = element('time', 'comment__time comment__time--meta');
      var reply = element('button', 'comment__reply');
      reply.setAttribute('type', 'button');
      Pinalert.setText(reply, REPLY_LABEL);
      // Like, Delete and Flag (plan 08-17, D-02, D-14): the thumbs up with its
      // count, then Delete on your own comment or Flag on someone else's.
      var like = element('button', 'comment__like');
      like.setAttribute('type', 'button');
      like.setAttribute('aria-pressed', 'false');
      var likeIcon = element('span', 'comment__like-icon');
      likeIcon.setAttribute('aria-hidden', 'true');
      var likeCount = element('span', 'comment__like-count');
      likeCount.hidden = true;
      like.appendChild(likeIcon);
      like.appendChild(likeCount);
      // Comments closed: the like turns into plain text, the icon and the number.
      var likes = element('span', 'comment__likes');
      likes.setAttribute('role', 'img');
      likes.hidden = true;
      var likesIcon = element('span', 'comment__like-icon');
      likesIcon.setAttribute('aria-hidden', 'true');
      var likesCount = element('span', 'comment__like-count');
      likes.appendChild(likesIcon);
      likes.appendChild(likesCount);
      var del = element('button', 'comment__delete');
      del.setAttribute('type', 'button');
      del.setAttribute('aria-label', DELETE_NAME);
      Pinalert.setText(del, DELETE_LABEL);
      del.hidden = true;
      var flag = element('button', 'comment__flag');
      flag.setAttribute('type', 'button');
      flag.setAttribute('aria-label', FLAG_NAME);
      Pinalert.setText(flag, FLAG_LABEL);
      flag.hidden = true;
      var held = element('span', 'comment__held');
      Pinalert.setText(held, HELD_LINE);
      held.hidden = true;
      meta.appendChild(timeMeta);
      meta.appendChild(reply);
      meta.appendChild(like);
      meta.appendChild(likes);
      meta.appendChild(del);
      meta.appendChild(flag);
      meta.appendChild(held);
      var actionError = element('div', 'inline-error comment__error');
      actionError.setAttribute('role', 'alert');
      actionError.hidden = true;
      var actionErrorText = element('span', 'comment__error-text');
      actionError.appendChild(actionErrorText);
      body.appendChild(head);
      body.appendChild(words);
      body.appendChild(gone);
      body.appendChild(meta);
      body.appendChild(actionError);
      rowEl.appendChild(disc);
      rowEl.appendChild(body);
      li.appendChild(rowEl);
      var rec = {
        id: String(data.id), isReply: isReply, rootId: String(rootId), data: data, li: li, disc: disc, discKey: '',
        head: head, author: author, tag: tag, timeHead: timeHead, words: words, gone: gone, meta: meta,
        timeMeta: timeMeta, reply: reply, held: held, wordsKey: null,
        like: like, likeCount: likeCount, likes: likes, likesCount: likesCount, del: del, flag: flag,
        error: actionError, errorText: actionErrorText,
        likeBusy: false, deleteBusy: false, flagSent: false,
        thread: null
      };
      reply.addEventListener('click', function (evt) {
        evt.stopPropagation();
        startReply(rec);
      });
      like.addEventListener('click', function (evt) {
        evt.stopPropagation();
        onLike(rec);
      });
      del.addEventListener('click', function (evt) {
        evt.stopPropagation();
        onDelete(rec);
      });
      flag.addEventListener('click', function (evt) {
        evt.stopPropagation();
        onFlag(rec);
      });
      if (!isReply) {
        buildThread(rec);
      }
      return rec;
    }

    function buildThread(rec) {
      var wrap = element('div', 'comment__replies');
      wrap.hidden = true;
      var ul = element('ul', 'comment-thread');
      ul.id = uid + '-thread-' + rec.id;
      ul.hidden = true;
      var toggle = element('button', 'comment-thread__toggle');
      toggle.setAttribute('type', 'button');
      toggle.setAttribute('aria-expanded', 'false');
      toggle.setAttribute('aria-controls', ul.id);
      // Show more replies reads the rest of a thread longer than the server's
      // reply cap (review fix round 2, SK-01). It shows only while the thread
      // is open, under its replies, so replies stay folded.
      var more = element('button', 'comment-thread__more');
      more.setAttribute('type', 'button');
      Pinalert.setText(more, SHOW_MORE_REPLIES);
      more.hidden = true;
      var moreErr = element('p', 'inline-error comment-thread__more-error');
      Pinalert.setText(moreErr, MORE_REPLIES_ERROR);
      moreErr.setAttribute('role', 'alert');
      moreErr.hidden = true;
      wrap.appendChild(ul);
      wrap.appendChild(more);
      wrap.appendChild(moreErr);
      wrap.appendChild(toggle);
      rec.li.appendChild(wrap);
      // total is the thread's real reply count (reply_count, plus your own
      // new reply until the next answer); coveredTo is the last reply id the
      // server's answers have covered from the start; serverMore says replies
      // after coveredTo are still to be read.
      rec.thread = {
        wrap: wrap, ul: ul, toggle: toggle, more: more, moreError: moreErr, replyIds: [],
        total: 0, coveredTo: 0, headLast: 0, serverMore: false, busy: false
      };
      toggle.addEventListener('click', function (evt) {
        evt.stopPropagation();
        openThreads[rec.id] = !openThreads[rec.id];
        writeThread(rec);
      });
      more.addEventListener('click', function (evt) {
        evt.stopPropagation();
        loadMoreReplies(rec);
      });
    }

    // writeWords puts the words into the paragraph through text nodes. A reply
    // whose words start with exactly '@' and the name the server sent as
    // reply_to_author, then a space, gets that name in a strong element; any
    // other leading @word stays plain text (review W3).
    function writeWords(rec, data) {
      var words = typeof data.words === 'string' ? data.words : '';
      var name = typeof data.reply_to_author === 'string' ? data.reply_to_author : '';
      var lead = name === '' ? '' : '@' + name + ' ';
      var key = (rec.isReply && lead !== '' && words.indexOf(lead) === 0 ? 'mention:' + name + ':' : 'plain:') + words;
      if (rec.wordsKey === key) {
        return;
      }
      rec.wordsKey = key;
      var p = rec.words;
      if (rec.isReply && lead !== '' && words.indexOf(lead) === 0) {
        Pinalert.setText(p, '');
        var strong = element('strong', 'comment__mention');
        Pinalert.setText(strong, '@' + name);
        p.appendChild(strong);
        p.appendChild(document.createTextNode(words.slice(lead.length - 1)));
      } else {
        Pinalert.setText(p, words);
      }
    }

    function writeComment(rec, data) {
      rec.data = data;
      var placeholder = data.deleted === true || data.removed === true || data.hidden === true;
      if (data.is_own === true && data.author) {
        ownName = String(data.author);
      }
      writeDisc(rec, data);
      rec.head.hidden = placeholder;
      rec.words.hidden = placeholder;
      rec.meta.hidden = placeholder;
      rec.gone.hidden = !placeholder;
      if (placeholder) {
        writeText(rec.gone, data.removed === true ? REMOVED_LINE
          : data.hidden === true ? HIDDEN_LINE : DELETED_LINE);
        // A placeholder carries no Like, no count, no Delete and no Flag, and
        // a poll never adds them back (review W4).
        rec.error.hidden = true;
        return;
      }
      writeText(rec.author, String(data.author || ''));
      rec.tag.hidden = data.is_reporter !== true;
      var when = data.at ? Pinalert.relativeTime(data.at) : '';
      writeText(rec.timeHead, when);
      writeText(rec.timeMeta, when);
      var iso = typeof data.at === 'string' ? data.at : '';
      rec.timeHead.setAttribute('datetime', iso);
      rec.timeMeta.setAttribute('datetime', iso);
      writeWords(rec, data);
      var held = data.held === true;
      rec.held.hidden = !held;
      rec.reply.hidden = held || !commentsOpen;
      writeActions(rec);
    }

    // writeActions shows Like, the closed count, Delete and Flag from what the
    // server last sent, and from nothing the client guessed: the count and the
    // pressed state are the server's own answer (threat T-08-76).
    function writeActions(rec) {
      var data = rec.data;
      var held = data.held === true;
      var count = typeof data.likes === 'number' && data.likes > 0 ? data.likes : 0;
      var liked = data.viewer_liked === true;
      rec.like.hidden = held || !commentsOpen;
      rec.like.setAttribute('aria-pressed', liked ? 'true' : 'false');
      rec.like.setAttribute('aria-label', likeName(count));
      writeClass(rec.like, liked ? 'comment__like comment__like--on' : 'comment__like');
      if (rec.likeBusy) {
        rec.like.setAttribute('aria-disabled', 'true');
      } else {
        rec.like.removeAttribute('aria-disabled');
      }
      rec.likeCount.hidden = count === 0;
      writeText(rec.likeCount, count > 0 ? String(count) : '');
      rec.likes.hidden = held || commentsOpen || count === 0;
      rec.likes.setAttribute('aria-label', likesName(count));
      writeText(rec.likesCount, count > 0 ? String(count) : '');
      // Delete is the author's; Flag is everyone else's. A held comment is
      // only ever the author's own, so it keeps Delete and has no Flag.
      var mine = data.can_delete === true;
      rec.del.hidden = !mine;
      if (rec.deleteBusy) {
        rec.del.setAttribute('aria-disabled', 'true');
      } else {
        rec.del.removeAttribute('aria-disabled');
      }
      rec.flag.hidden = mine || held || data.is_own === true;
      var sent = rec.flagSent || data.viewer_flagged === true;
      if (sent) {
        rec.flag.setAttribute('aria-disabled', 'true');
        rec.flag.removeAttribute('aria-label');
        writeText(rec.flag, FLAGGED_LABEL);
        writeClass(rec.flag, 'comment__flag comment__flag--sent');
      } else {
        rec.flag.removeAttribute('aria-disabled');
        rec.flag.setAttribute('aria-label', FLAG_NAME);
        writeText(rec.flag, FLAG_LABEL);
        writeClass(rec.flag, 'comment__flag');
      }
    }

    function showActionError(rec, words) {
      writeText(rec.errorText, words);
      rec.error.hidden = false;
    }

    function clearActionError(rec) {
      rec.error.hidden = true;
    }

    // actionFailed words a refused or failed like, unlike or delete: the
    // server's own sentence when it sent one (the rate limit's), the offline
    // sentence when the device has no signal, a short line otherwise.
    function actionFailed(rec, res) {
      var message = res && res.body && res.body.error && typeof res.body.error.message === 'string' ? res.body.error.message : '';
      var field = res && res.body && res.body.error && typeof res.body.error.field === 'string' ? res.body.error.field : '';
      if (res && res.status === STATUS_CONFLICT && field === 'comments') {
        applyOpenState(false);
        return;
      }
      if (res && res.status === STATUS_GONE) {
        load();
        return;
      }
      if (message !== '' && res && (res.status === 429 || res.status === 403)) {
        showActionError(rec, message);
        return;
      }
      showActionError(rec, navigator.onLine === false ? OFFLINE_ACTION : ACTION_ERROR);
    }

    function commentPath(rec, verb) {
      return '/api/comments/' + encodeURIComponent(rec.id) + '/' + verb;
    }

    function onLike(rec) {
      if (rec.likeBusy || rec.data.deleted || rec.data.removed || rec.data.hidden) {
        return;
      }
      clearActionError(rec);
      if (navigator.onLine === false) {
        showActionError(rec, OFFLINE_ACTION);
        return;
      }
      var verb = rec.data.viewer_liked === true ? 'unlike' : 'like';
      rec.likeBusy = true;
      writeActions(rec);
      request('POST', commentPath(rec, verb)).promise.then(function (res) {
        rec.likeBusy = false;
        if (res.status === STATUS_OK && res.body && typeof res.body.likes === 'number') {
          rec.data.likes = res.body.likes;
          rec.data.viewer_liked = res.body.viewer_liked === true;
          answerStamp += 1;
          rec.likeStamp = answerStamp;
          writeActions(rec);
          return;
        }
        writeActions(rec);
        actionFailed(rec, res);
      }).catch(function () {
        rec.likeBusy = false;
        writeActions(rec);
        showActionError(rec, navigator.onLine === false ? OFFLINE_ACTION : ACTION_ERROR);
      });
    }

    // onDelete asks first, in the app's confirm box; only its Delete button
    // sends anything.
    function onDelete(rec) {
      if (rec.deleteBusy || rec.data.deleted || rec.data.removed || rec.data.hidden) {
        return;
      }
      clearActionError(rec);
      var asked = { opener: rec.del, answered: false, run: function () {
        sendDelete(rec);
      } };
      if (!openDeleteBox(asked)) {
        if (typeof window.confirm === 'function' && window.confirm(DELETE_QUESTION)) {
          sendDelete(rec);
        }
      }
    }

    function sendDelete(rec) {
      if (navigator.onLine === false) {
        showActionError(rec, OFFLINE_ACTION);
        rec.del.focus();
        return;
      }
      rec.deleteBusy = true;
      writeActions(rec);
      request('POST', commentPath(rec, 'delete')).promise.then(function (res) {
        rec.deleteBusy = false;
        if (res.status === STATUS_OK) {
          afterDelete(rec);
          return;
        }
        writeActions(rec);
        rec.del.focus();
        actionFailed(rec, res);
      }).catch(function () {
        rec.deleteBusy = false;
        writeActions(rec);
        rec.del.focus();
        showActionError(rec, navigator.onLine === false ? OFFLINE_ACTION : ACTION_ERROR);
      });
    }

    // afterDelete: a comment with replies stays as "Comment deleted" so the
    // thread still reads; one without disappears. Focus goes to the heading.
    function afterDelete(rec) {
      var hasReplies = !rec.isReply && rec.thread !== null && rec.thread.replyIds.length > 0;
      if (hasReplies) {
        rec.data.deleted = true;
        rec.data.likes = 0;
        rec.data.viewer_liked = false;
        rec.data.is_reporter = false;
        writeComment(rec, rec.data);
      } else if (rec.isReply) {
        removeReply(rec);
      } else {
        removeRoot(rec.id);
      }
      if (total > 0) {
        total -= 1;
      }
      syncParts();
      speakDeleted();
      heading.focus({ preventScroll: true });
      load();
    }

    function removeReply(rec) {
      var parent = nodes[rec.rootId];
      if (replyTarget && replyTarget.replyToId === rec.id) {
        clearReplyTarget(false);
      }
      if (rec.li.parentNode) {
        rec.li.parentNode.removeChild(rec.li);
      }
      delete nodes[rec.id];
      if (parent && parent.thread) {
        var at = parent.thread.replyIds.indexOf(rec.id);
        if (at >= 0) {
          parent.thread.replyIds.splice(at, 1);
        }
        if (parent.thread.total > 0) {
          parent.thread.total -= 1;
        }
        writeThread(parent);
      }
    }

    // onFlag opens the built Flag dialog in comment mode; the dialog sends the
    // flag (plan 08-18), so this control never does. Once sent it reads Flagged.
    function onFlag(rec) {
      if (rec.flag.getAttribute('aria-disabled') === 'true') {
        return;
      }
      if (!window.PinalertFlag || typeof window.PinalertFlag.openForComment !== 'function') {
        return;
      }
      window.PinalertFlag.openForComment({
        commentId: rec.id,
        opener: rec.flag,
        onSent: function () {
          rec.flagSent = true;
          writeActions(rec);
        }
      });
    }

    function writeThread(rec) {
      var t = rec.thread;
      if (!t) {
        return;
      }
      // The toggle counts the thread's real replies, which can be more than
      // the replies read so far (review fix round 2, SK-01).
      var count = Math.max(t.total, t.replyIds.length);
      t.wrap.hidden = count === 0;
      if (count === 0) {
        return;
      }
      var open = openThreads[rec.id] === true;
      t.ul.hidden = !open;
      t.toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
      writeText(t.toggle, open ? HIDE_REPLIES : viewReplies(count));
      var offer = open && t.serverMore;
      t.more.hidden = !offer;
      if (!offer) {
        t.moreError.hidden = true;
      }
    }

    // freshData keeps the like state a Like answer wrote when the list answer
    // carrying data was asked for before that Like answered (review fix
    // WR-B05): an older server answer never overwrites a newer one. sentStamp
    // is answerStamp at the moment the list request left.
    function freshData(rec, data, sentStamp) {
      if (!rec || !(rec.likeStamp > sentStamp)) {
        return data;
      }
      var kept = {};
      Object.keys(data).forEach(function (key) {
        kept[key] = data[key];
      });
      kept.likes = rec.data.likes;
      kept.viewer_liked = rec.data.viewer_liked;
      return kept;
    }

    // placeReplies writes one answer's replies into the thread, in id order
    // (the server's order, oldest first), writing the nodes it already has and
    // adding the rest. A reply the answer should have held but did not is
    // dropped only inside the range the answer covers: after from (0 for the
    // list) up to its last reply, or to the end when toEnd is set because the
    // answer holds the whole rest of the thread (review fix round 2, SK-01).
    // Replies after that range stay, and so does your own reply posted after
    // the request left, so a capped thread never loses the replies it did not
    // cover and a new reply never vanishes at the next poll.
    function placeReplies(rec, replies, sentStamp, from, toEnd) {
      var t = rec.thread;
      var lo = typeof from === 'number' ? from : 0;
      var seen = {};
      var lastId = lo;
      for (var i = 0; i < replies.length; i++) {
        var id = String(replies[i].id);
        var data = freshData(nodes[id], replies[i], sentStamp);
        seen[id] = true;
        lastId = Math.max(lastId, Number(data.id));
        var child = nodes[id];
        if (!child) {
          child = makeRecord(data, true, rec.id);
          nodes[id] = child;
        }
        writeComment(child, data);
      }
      var hi = toEnd ? Infinity : lastId;
      var ids = [];
      for (var j = 0; j < t.replyIds.length; j++) {
        var old = t.replyIds[j];
        if (seen[old]) {
          continue;
        }
        var n = Number(old);
        var postedSince = nodes[old] && nodes[old].postStamp > sentStamp;
        if (n > lo && n <= hi && !postedSince) {
          if (replyTarget && replyTarget.replyToId === old) {
            clearReplyTarget(false);
          }
          if (nodes[old] && nodes[old].li.parentNode) {
            nodes[old].li.parentNode.removeChild(nodes[old].li);
          }
          delete nodes[old];
          continue;
        }
        ids.push(old);
      }
      for (var k = 0; k < replies.length; k++) {
        ids.push(String(replies[k].id));
      }
      ids.sort(function (a, b) { return Number(a) - Number(b); });
      var prev = null;
      for (var m = 0; m < ids.length; m++) {
        var node = nodes[ids[m]];
        var at = prev === null ? t.ul.firstChild : prev.li.nextSibling;
        if (node.li.parentNode !== t.ul || at !== node.li) {
          t.ul.insertBefore(node.li, at);
        }
        prev = node;
      }
      t.replyIds = ids;
      writeThread(rec);
    }

    // coverReplies notes what a list answer said about a thread: its real
    // reply count, and whether replies after the ones it carried are left to
    // read. A thread the answer holds whole has none left. A capped one has
    // some when its count is more than the replies read, or when the answer
    // reached past every page Show more replies read (SK-01).
    function coverReplies(rec, data, replies, capped) {
      var t = rec.thread;
      t.total = typeof data.reply_count === 'number' ? data.reply_count : 0;
      var last = replies.length > 0 ? Number(replies[replies.length - 1].id) : 0;
      if (!capped) {
        t.coveredTo = last;
        t.serverMore = false;
        t.headLast = 0;
      } else {
        // An open thread whose tail was read before has that tail read again
        // after this answer, because counts alone cannot tell a reply that
        // went from a new one (review fix round 3, SK2-01).
        if (last > 0 && t.coveredTo > last && openThreads[rec.id] === true) {
          pendingTails.push(rec);
        }
        t.serverMore = t.total > t.replyIds.length || last >= t.coveredTo;
        t.coveredTo = Math.max(t.coveredTo, last);
        t.headLast = last;
      }
      writeThread(rec);
    }

    // rereadTails reads again the tail of each open capped thread the answer
    // just written (the replies after the first page), through placeReplies
    // over the range the read covers. A reply gone from the tail leaves the
    // screen, a new one lands in id order, and Show more replies stays only
    // when the read itself left replies after it. A failed read leaves the
    // thread as the poll wrote it.
    function rereadTails() {
      var recs = pendingTails;
      pendingTails = [];
      for (var i = 0; i < recs.length; i++) {
        rereadTail(recs[i]);
      }
    }

    function rereadTail(rec) {
      var t = rec.thread;
      if (!t || t.busy || nodes[rec.id] !== rec || !(t.headLast > 0)) {
        return;
      }
      var after = t.headLast;
      var sentStamp = answerStamp;
      var call = request('GET', threadPath(rec.id, after));
      call.promise.then(function (res) {
        if (nodes[rec.id] !== rec || t.headLast !== after || t.busy) {
          return;
        }
        if (!res.ok || !res.body || !Array.isArray(res.body.comments)) {
          return;
        }
        var head = res.body.comments[0];
        if (!head || String(head.id) !== String(rec.id)) {
          return;
        }
        var replies = Array.isArray(head.replies) ? head.replies : [];
        var more = res.body.more === true;
        placeReplies(rec, replies, sentStamp, after, !more);
        if (typeof head.reply_count === 'number') {
          t.total = head.reply_count;
        }
        var next = typeof res.body.next_after === 'number' ? res.body.next_after : 0;
        t.coveredTo = Math.max(after, next);
        t.serverMore = more;
        writeThread(rec);
      }).catch(function () {});
    }

    function removeRoot(id) {
      var rec = nodes[id];
      if (!rec) {
        return;
      }
      if (replyTarget && replyTarget.rootId === id) {
        clearReplyTarget(false);
      }
      if (rec.thread) {
        for (var i = 0; i < rec.thread.replyIds.length; i++) {
          delete nodes[rec.thread.replyIds[i]];
        }
      }
      if (rec.li.parentNode) {
        rec.li.parentNode.removeChild(rec.li);
      }
      delete nodes[id];
      rootIds.splice(rootIds.indexOf(id), 1);
    }

    // insertRoot puts a new top level comment into the list by its id. The
    // server sends top level comments oldest first and pages them by id, so id
    // order is the list's order: a later page, or your own post on a report
    // with pages still unread, lands in its place (review fix WR-B04).
    function insertRoot(rec) {
      var n = Number(rec.id);
      var at = rootIds.length;
      for (var i = 0; i < rootIds.length; i++) {
        if (Number(rootIds[i]) > n) {
          at = i;
          break;
        }
      }
      var next = at < rootIds.length && nodes[rootIds[at]] ? nodes[rootIds[at]].li : null;
      rootIds.splice(at, 0, rec.id);
      list.insertBefore(rec.li, next);
    }

    // mergeRoots writes one page into the list: known comments are updated,
    // new ones inserted in id order, and comments that the answer should have
    // held but did not (a held comment that failed its check, a deleted one)
    // are dropped, only inside the range the answer covers: after the page's
    // cursor (after, 0 for the first page) up to its last comment, or to the
    // end when no page follows.
    function mergeRoots(comments, covered, after, sentStamp) {
      var from = typeof after === 'number' ? after : 0;
      var seen = {};
      var lastId = from;
      for (var i = 0; i < comments.length; i++) {
        var id = String(comments[i].id);
        var data = freshData(nodes[id], comments[i], sentStamp);
        seen[id] = true;
        lastId = Math.max(lastId, Number(data.id));
        var rec = nodes[id];
        if (!rec) {
          rec = makeRecord(data, false, id);
          nodes[id] = rec;
          insertRoot(rec);
        }
        writeComment(rec, data);
        var replies = Array.isArray(data.replies) ? data.replies : [];
        // The server lists at most its reply cap of each thread, oldest
        // first; reply_count is the real count (review fix round 2, SK-01).
        var capped = typeof data.reply_count === 'number' && data.reply_count > replies.length;
        placeReplies(rec, replies, sentStamp, 0, !capped);
        coverReplies(rec, data, replies, capped);
      }
      var limit = covered === 'all' ? Infinity : lastId;
      var stale = rootIds.filter(function (id) {
        return !seen[id] && Number(id) > from && Number(id) <= limit;
      });
      for (var j = 0; j < stale.length; j++) {
        removeRoot(stale[j]);
      }
    }

    // ---- the list area ----

    function clearList() {
      nodes = {};
      rootIds = [];
      // A cleared list starts again from the first page.
      loadedMore = false;
      lastPageAfter = 0;
      while (list.firstChild) {
        list.removeChild(list.firstChild);
      }
    }

    function writeCount() {
      countNode.hidden = !(total > 0);
      writeText(countNode, total > 0 ? String(total) : '');
    }

    // syncParts shows the pieces the mode and the report's state call for.
    function syncParts() {
      var rows = mode === 'rows';
      list.hidden = !(rows && rootIds.length > 0) && mode !== 'skeleton';
      emptyLine.hidden = !(rows && rootIds.length === 0 && commentsOpen);
      closedLine.hidden = !(rows && !commentsOpen);
      form.hidden = !(rows && commentsOpen);
      moreButton.hidden = !(rows && more);
      if (!(rows && more)) {
        moreError.hidden = true;
      }
      errorBlock.hidden = mode !== 'error';
      list.setAttribute('aria-busy', mode === 'skeleton' ? 'true' : 'false');
      writeCount();
    }

    function showSkeleton() {
      if (mode === 'skeleton') {
        return;
      }
      mode = 'skeleton';
      clearList();
      for (var i = 0; i < SKELETON_COMMENTS; i++) {
        var li = element('li', 'comment comment--skeleton');
        li.appendChild(element('span', 'comment__disc comment__disc--empty'));
        var body = element('div', 'comment__body');
        body.appendChild(element('span', 'comment__bar comment__bar--head'));
        body.appendChild(element('span', 'comment__bar comment__bar--words'));
        li.appendChild(body);
        list.appendChild(li);
      }
      syncParts();
    }

    function showError(line) {
      mode = 'error';
      clearList();
      writeText(errorTitle, ERROR_TITLE);
      writeText(errorLine, line);
      syncParts();
    }

    function hideSection() {
      root.hidden = true;
    }

    function applyOpenState(next) {
      if (next === commentsOpen) {
        return;
      }
      commentsOpen = next;
      if (!commentsOpen) {
        clearReplyTarget(false);
      }
      Object.keys(nodes).forEach(function (id) {
        writeComment(nodes[id], nodes[id].data);
      });
      syncParts();
    }

    function render(body, sentStamp) {
      var comments = body && Array.isArray(body.comments) ? body.comments : null;
      if (comments === null) {
        throw new Error('the comments answer has no comments');
      }
      if (mode !== 'rows') {
        clearList();
        mode = 'rows';
      }
      if (typeof body.comment_count === 'number') {
        total = body.comment_count;
      }
      applyOpenState(body.comments_open !== false);
      mergeRoots(comments, body.more === true ? 'range' : 'all', 0, sentStamp);
      rereadTails();
      if (!loadedMore) {
        more = body.more === true;
        nextAfter = typeof body.next_after === 'number' ? body.next_after : 0;
      }
      syncParts();
      writeOwnDisc();
    }

    // renderTail writes the last page read so far, from Show more or from the
    // poll once Show more has been used (review fix WR-B04). That page is the
    // list's end, so its answer says whether Show more is needed again.
    function renderTail(body, after, sentStamp) {
      if (typeof body.comment_count === 'number') {
        total = body.comment_count;
      }
      mergeRoots(body.comments, body.more === true ? 'range' : 'all', after, sentStamp);
      rereadTails();
      more = body.more === true;
      nextAfter = typeof body.next_after === 'number' ? body.next_after : 0;
      syncParts();
      writeOwnDisc();
    }

    // ---- requests ----

    // request makes one comments request under the shared time limit of app.js
    // (Pinalert.requestTimeout, review fix WR-B01): the page's own timer is
    // raced against the request and against its body read, so a request whose
    // aborted promise never settles (WebKit) or whose body stalls after the
    // headers still ends at COMMENTS_TIMEOUT_MS, and Post, Like, Delete and
    // Show more come back. The limit is cleared only once the body has come.
    function request(method, path, payload) {
      var limit = Pinalert.requestTimeout(COMMENTS_TIMEOUT_MS, 'comments');
      var opts = {
        method: method,
        credentials: 'same-origin',
        headers: { Accept: 'application/json' },
        signal: limit.signal
      };
      if (payload) {
        opts.headers['Content-Type'] = 'application/json';
        opts.body = JSON.stringify(payload);
      }
      var promise = Promise.race([fetch(path, opts), limit.expired]).then(function (res) {
        return Promise.race([res.json(), limit.expired]).then(function (body) {
          limit.clear();
          return { status: res.status, ok: res.ok, body: body };
        }, function () {
          limit.clear();
          return { status: res.status, ok: res.ok, body: null };
        });
      }, function (err) {
        limit.clear();
        throw err;
      });
      return { promise: promise, abort: function () {
        limit.clear();
        if (limit.controller) {
          limit.controller.abort();
        }
      } };
    }

    function listPath(after) {
      var path = '/api/reports/' + encodeURIComponent(reportId) + '/comments';
      return after ? path + '?after=' + encodeURIComponent(String(after)) : path;
    }

    // threadPath reads one thread's replies after the last one read (SK-01).
    function threadPath(rootId, after) {
      var path = '/api/reports/' + encodeURIComponent(reportId) + '/comments?replies_of=' + encodeURIComponent(String(rootId));
      return after ? path + '&reply_after=' + encodeURIComponent(String(after)) : path;
    }

    function stopLoad() {
      loadSeq += 1;
      if (inflight !== null) {
        inflight.abort();
        inflight = null;
      }
    }

    // load asks for the first page. The first answer shows a skeleton; a later
    // one is written over the nodes already on the page, and a failed later one
    // leaves them alone.
    function load() {
      if (!active) {
        return;
      }
      stopLoad();
      var mine = loadSeq;
      if (navigator.onLine === false) {
        if (mode !== 'rows') {
          showError(OFFLINE_LINE);
        }
        return;
      }
      if (mode !== 'rows') {
        showSkeleton();
      }
      var sentStamp = answerStamp;
      inflight = request('GET', listPath(0));
      inflight.promise.then(function (res) {
        if (mine !== loadSeq) {
          return;
        }
        inflight = null;
        if (res.status === STATUS_GONE) {
          hideSection();
          return;
        }
        if (!res.ok) {
          throw new Error('the comments answered with an error');
        }
        root.hidden = false;
        render(res.body, sentStamp);
        loadTail(mine);
      }).catch(function () {
        if (mine !== loadSeq) {
          return;
        }
        inflight = null;
        if (mode !== 'rows') {
          showError(navigator.onLine === false ? OFFLINE_LINE : ERROR_LINE);
        }
      });
    }

    // loadTail is the second read of a poll once Show more has been used: the
    // last page read so far, where every new top level comment lands (review
    // fix WR-B04). It takes the poll's place in inflight, so stopLoad ends it,
    // and a failed read leaves the nodes alone.
    function loadTail(mine) {
      if (!loadedMore || lastPageAfter <= 0 || mine !== loadSeq || !active) {
        return;
      }
      var after = lastPageAfter;
      var sentStamp = answerStamp;
      inflight = request('GET', listPath(after));
      inflight.promise.then(function (res) {
        if (mine !== loadSeq) {
          return;
        }
        inflight = null;
        if (!res.ok || !res.body || !Array.isArray(res.body.comments) || after !== lastPageAfter) {
          return;
        }
        renderTail(res.body, after, sentStamp);
      }).catch(function () {
        if (mine === loadSeq) {
          inflight = null;
        }
      });
    }

    var moreBusy = false;
    function loadMore() {
      if (moreBusy || !more) {
        return;
      }
      moreBusy = true;
      moreError.hidden = true;
      moreButton.setAttribute('aria-disabled', 'true');
      var after = nextAfter;
      var sentStamp = answerStamp;
      var call = request('GET', listPath(after));
      call.promise.then(function (res) {
        moreBusy = false;
        moreButton.removeAttribute('aria-disabled');
        if (!res.ok || !res.body || !Array.isArray(res.body.comments)) {
          throw new Error('more comments failed');
        }
        loadedMore = true;
        lastPageAfter = after;
        renderTail(res.body, after, sentStamp);
      }).catch(function () {
        moreBusy = false;
        moreButton.removeAttribute('aria-disabled');
        moreError.hidden = false;
      });
    }

    // loadMoreReplies reads the next page of one thread's replies, after the
    // last reply the server's answers have covered, and writes it through
    // placeReplies (review fix round 2, SK-01). A thread no longer on the list
    // when the answer comes is left alone; a failed read keeps the button and
    // says so, and a second press asks again.
    function loadMoreReplies(rec) {
      var t = rec.thread;
      if (!t || t.busy || !t.serverMore) {
        return;
      }
      t.busy = true;
      t.moreError.hidden = true;
      t.more.setAttribute('aria-disabled', 'true');
      var after = t.coveredTo;
      var sentStamp = answerStamp;
      var call = request('GET', threadPath(rec.id, after));
      call.promise.then(function (res) {
        t.busy = false;
        t.more.removeAttribute('aria-disabled');
        if (nodes[rec.id] !== rec) {
          return;
        }
        if (!res.ok || !res.body || !Array.isArray(res.body.comments)) {
          throw new Error('more replies failed');
        }
        var head = res.body.comments[0];
        if (!head || String(head.id) !== String(rec.id)) {
          t.serverMore = false;
          writeThread(rec);
          return;
        }
        var replies = Array.isArray(head.replies) ? head.replies : [];
        var more = res.body.more === true;
        placeReplies(rec, replies, sentStamp, after, !more);
        if (typeof head.reply_count === 'number') {
          t.total = head.reply_count;
        }
        var next = typeof res.body.next_after === 'number' ? res.body.next_after : 0;
        t.coveredTo = Math.max(t.coveredTo, next);
        t.serverMore = more;
        writeThread(rec);
      }).catch(function () {
        t.busy = false;
        t.more.removeAttribute('aria-disabled');
        if (nodes[rec.id] === rec) {
          t.moreError.hidden = false;
        }
      });
    }

    function tick() {
      if (!active || document.visibilityState !== 'visible') {
        return;
      }
      load();
    }

    // onVisible asks for the list again when the tab comes back, so a reader
    // back from another app does not wait for the next tick (review fix
    // IN-B06). It is listened for only while the poll runs.
    function onVisible() {
      tick();
    }

    function startPolling() {
      if (pollTimer === null) {
        pollTimer = window.setInterval(tick, COMMENTS_POLL_MS);
        if (typeof document.addEventListener === 'function') {
          document.addEventListener('visibilitychange', onVisible);
        }
      }
    }

    function stopPolling() {
      if (pollTimer !== null) {
        window.clearInterval(pollTimer);
        pollTimer = null;
        if (typeof document.removeEventListener === 'function') {
          document.removeEventListener('visibilitychange', onVisible);
        }
      }
    }

    // ---- the box ----

    function writeOwnDisc() {
      var first = Array.from(ownName)[0] || '';
      var cls = first === '' ? 'comment__disc comment__disc--own comment__disc--glyph' : 'comment__disc comment__disc--own';
      writeClass(ownDisc, cls);
      writeText(ownDisc, first.toUpperCase());
    }

    function remaining() {
      return COMMENT_MAX - codePoints(input.value);
    }

    function speak(words) {
      Pinalert.setText(status, '');
      Pinalert.setText(status, words);
    }

    function writeCounter() {
      var left = remaining();
      counter.hidden = left > COUNTER_FROM;
      if (left > COUNTER_FROM) {
        spokenBand = Infinity;
        return;
      }
      writeText(counter, leftWords(left));
      var band = left <= COUNTER_END ? COUNTER_END : (left <= COUNTER_NEAR ? COUNTER_NEAR : COUNTER_FROM);
      if (band < spokenBand) {
        speak(leftWords(left));
      }
      spokenBand = band;
    }

    function writePostState() {
      var on = input.value.trim() !== '' && !posting;
      post.setAttribute('aria-disabled', on ? 'false' : 'true');
      writeText(post, posting ? POSTING_LABEL : POST_LABEL);
    }

    function autosize() {
      if (input.style) {
        input.style.height = 'auto';
        var h = input.scrollHeight || 0;
        if (h > 0) {
          input.style.height = Math.min(h, INPUT_MAX_HEIGHT) + 'px';
        }
      }
    }

    function showComposerError(words) {
      writeText(composerError, words);
      composerError.hidden = false;
    }

    function clearComposerError() {
      composerError.hidden = true;
    }

    function onInput() {
      writeCounter();
      writePostState();
      autosize();
    }

    // startReply points the box at a comment. The leading @username goes in
    // only for a person shown by a username who is not you: not for your own
    // comment, New member or Deleted account (review W3).
    function startReply(rec) {
      var data = rec.data;
      if (!commentsOpen || data.deleted || data.removed || data.hidden || data.held) {
        return;
      }
      var mention = isUser(data) && data.is_own !== true ? '@' + data.author + ' ' : '';
      var value = input.value;
      if (replyTarget && replyTarget.mention && value.indexOf(replyTarget.mention) === 0) {
        value = value.slice(replyTarget.mention.length);
      }
      replyTarget = { rootId: rec.rootId, replyToId: rec.id, shown: String(data.author || ''), mention: mention };
      input.value = mention !== '' && value.indexOf(mention) === 0 ? value : mention + value;
      writeText(replyingName, replyTarget.shown);
      replying.hidden = false;
      input.setAttribute('aria-label', 'Reply to ' + replyTarget.shown);
      input.focus();
      if (input.setSelectionRange) {
        var at = mention !== '' ? mention.length : input.value.length;
        input.setSelectionRange(at, at);
      }
      onInput();
    }

    // clearReplyTarget drops the target. The mention it inserted goes only
    // when nothing was typed after it.
    function clearReplyTarget(refocus) {
      if (!replyTarget) {
        return;
      }
      var mention = replyTarget.mention;
      if (mention !== '' && input.value.indexOf(mention) === 0 && input.value.slice(mention.length).trim() === '') {
        input.value = input.value.slice(mention.length);
      }
      replyTarget = null;
      replying.hidden = true;
      input.setAttribute('aria-label', ASK_LABEL);
      onInput();
      if (refocus) {
        input.focus();
      }
    }

    // addPosted puts the comment the server just accepted into the list in
    // place. A reply goes into its thread and opens it.
    function addPosted(data, target) {
      if (data.held !== true) {
        total += 1;
      }
      if (target) {
        var rootRec = nodes[target.rootId];
        if (rootRec && rootRec.thread) {
          var child = makeRecord(data, true, rootRec.id);
          // The stamp keeps this reply through a list answer that left before
          // it was posted, and total counts it until the next answer does
          // (review fix round 2, SK-01).
          answerStamp += 1;
          child.postStamp = answerStamp;
          rootRec.thread.total = Math.max(rootRec.thread.total, rootRec.thread.replyIds.length) + 1;
          nodes[String(data.id)] = child;
          rootRec.thread.ul.appendChild(child.li);
          rootRec.thread.replyIds.push(String(data.id));
          writeComment(child, data);
          openThreads[rootRec.id] = true;
          writeThread(rootRec);
        }
      } else {
        var id = String(data.id);
        if (!nodes[id]) {
          var rec = makeRecord(data, false, id);
          nodes[id] = rec;
          insertRoot(rec);
          writeComment(rec, data);
        }
      }
      if (mode === 'rows') {
        syncParts();
      }
      writeOwnDisc();
    }

    // clearSent takes the posted words out of the box. Words typed while the
    // post was in flight stay (review fix WR-B03): only the sent words at the
    // start go, and a box that no longer starts with them is left alone. The
    // reply target goes only when it is still the one the post was sent to.
    function clearSent(sent, target) {
      if (input.value === sent) {
        input.value = '';
      } else if (sent !== '' && input.value.indexOf(sent) === 0) {
        input.value = input.value.slice(sent.length).replace(/^\s+/, '');
      }
      if (replyTarget === target) {
        clearReplyTarget(false);
      }
    }

    function onPostAnswer(res, target, sent) {
      var message = res.body && res.body.error && typeof res.body.error.message === 'string' ? res.body.error.message : '';
      var field = res.body && res.body.error && typeof res.body.error.field === 'string' ? res.body.error.field : '';
      if (res.status === STATUS_CREATED && res.body && typeof res.body === 'object') {
        clearSent(sent, target);
        clearComposerError();
        addPosted(res.body, target);
        onInput();
        speak(POSTED_STATUS);
        input.focus();
        load();
        return;
      }
      if (res.status === STATUS_CONFLICT && field === 'comments') {
        commentsOpen = true;
        applyOpenState(false);
        input.value = '';
        clearReplyTarget(false);
        onInput();
        return;
      }
      if (message !== '' && (res.status === 400 || res.status === 422 || res.status === 429 || res.status === STATUS_CONFLICT || res.status === STATUS_GONE)) {
        showComposerError(message);
        if (res.status === STATUS_GONE && field === 'comment') {
          load();
        }
        return;
      }
      showComposerError(POST_ERROR);
      // The outcome is unknown (an unreadable answer, a 201 whose body never
      // came): the list is asked for again, so a comment the server did store
      // shows before a second press posts it twice (review fix IN-B02).
      load();
    }

    function submit() {
      if (posting || input.value.trim() === '') {
        return;
      }
      clearComposerError();
      if (navigator.onLine === false) {
        showComposerError(OFFLINE_POST);
        return;
      }
      posting = true;
      writePostState();
      var sent = input.value;
      var payload = { words: sent };
      var target = replyTarget;
      if (target) {
        payload.reply_to = Number(target.replyToId);
      }
      request('POST', listPath(0), payload).promise.then(function (res) {
        posting = false;
        writePostState();
        onPostAnswer(res, target, sent);
      }).catch(function () {
        posting = false;
        writePostState();
        showComposerError(navigator.onLine === false ? OFFLINE_POST : POST_ERROR);
        load();
      });
    }

    // ---- listeners ----

    // Every key event in the box stops at the form: the map's own handlers and
    // Leaflet's keyboard handling never see typing.
    ['keydown', 'keypress', 'keyup'].forEach(function (type) {
      form.addEventListener(type, function (evt) {
        evt.stopPropagation();
      });
    });
    input.addEventListener('keydown', function (evt) {
      evt.stopPropagation();
      // Safari sends the Enter that ends a composition with isComposing false
      // and keyCode 229 for some input methods; that Enter never posts (review
      // fix IN-B07).
      if (evt.key === 'Enter' && evt.shiftKey !== true && evt.isComposing !== true && evt.keyCode !== 229) {
        evt.preventDefault();
        submit();
        return;
      }
      if (evt.key === 'Escape') {
        if (replyTarget) {
          evt.preventDefault();
          clearReplyTarget(false);
        }
      }
    });
    input.addEventListener('input', onInput);
    post.addEventListener('click', function (evt) {
      evt.stopPropagation();
      if (post.getAttribute('aria-disabled') === 'true') {
        return;
      }
      submit();
    });
    form.addEventListener('submit', function (evt) {
      evt.preventDefault();
      submit();
    });
    cancel.addEventListener('click', function (evt) {
      evt.stopPropagation();
      clearReplyTarget(true);
    });
    moreButton.addEventListener('click', function (evt) {
      evt.stopPropagation();
      if (moreButton.getAttribute('aria-disabled') === 'true') {
        return;
      }
      loadMore();
    });
    retry.addEventListener('click', function (evt) {
      evt.stopPropagation();
      if (!active) {
        return;
      }
      // The button is about to leave the page, so focus goes to the heading;
      // this is a reader's own press, not a refresh.
      heading.focus({ preventScroll: true });
      mode = 'none';
      load();
    });
    // Enter and Space on a control act on the control, never on the map.
    root.addEventListener('keydown', function (evt) {
      if (evt.key === 'Enter' || evt.key === ' ' || evt.key === 'Spacebar') {
        evt.stopPropagation();
      }
    });

    writeOwnDisc();
    writePostState();
    syncParts();

    // ---- the page object ----

    // update records what the card now shows. Nothing is rebuilt: a change of
    // the open flag rewrites the nodes it touches, and a changed count asks
    // for the list again when the section is showing.
    function update(next) {
      current = next;
      if (typeof next.comments_open === 'boolean' && mode === 'rows') {
        applyOpenState(next.comments_open);
      }
      if (typeof next.comment_count === 'number' && next.comment_count !== lastFeedCount) {
        lastFeedCount = next.comment_count;
        if (active) {
          load();
        }
      }
    }

    function open() {
      active = true;
      startPolling();
      load();
    }

    function close() {
      active = false;
      stopPolling();
      stopLoad();
    }

    function refresh() {
      if (active) {
        load();
      }
    }

    return { el: root, update: update, open: open, close: close, refresh: refresh };
  }

  return { create: create };
})();
