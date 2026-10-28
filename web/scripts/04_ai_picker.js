    function closePicker(triggerID, optionsID) {
      document.getElementById(optionsID).hidden = true;
      document.getElementById(triggerID).setAttribute('aria-expanded', 'false');
    }

    function togglePicker(triggerID, optionsID) {
      const options = document.getElementById(optionsID);
      if (!options.hidden) { closePicker(triggerID, optionsID); return; }
      options.hidden = false;
      document.getElementById(triggerID).setAttribute('aria-expanded', 'true');
      const selected = options.querySelector('[aria-selected="true"]') || options.firstElementChild;
      selected?.focus();
    }

    function pickerTriggerKey(event, triggerID, optionsID) {
      if (!['ArrowDown', 'ArrowUp'].includes(event.key)) return;
      event.preventDefault();
      togglePicker(triggerID, optionsID);
    }

    function closeEnginePicker() { closePicker('engineContainer', 'engineOptions'); }
    function toggleEnginePicker() { togglePicker('engineContainer', 'engineOptions'); }
    function engineTriggerKey(event) {
      pickerTriggerKey(event, 'engineContainer', 'engineOptions');
    }

    function engineOptionKey(event) {
      const listbox = event.currentTarget.parentElement;
      const trigger = document.querySelector(`[aria-controls="${listbox.id}"]`);
      const options = [...listbox.children];
      const index = options.indexOf(event.currentTarget);
      const positions = {
        ArrowDown: (index + 1) % options.length,
        ArrowUp: (index + options.length - 1) % options.length,
        Home: 0, End: options.length - 1
      };
      if (event.key in positions) {
        event.preventDefault();
        options[positions[event.key]].focus();
      }
      if (event.key === 'Escape' || event.key === 'Tab') {
        closePicker(trigger.id, listbox.id);
        if (event.key === 'Escape') { event.preventDefault(); trigger.focus(); }
      }
    }

    for (const [pickerID, triggerID, optionsID] of [
      ['enginePicker', 'engineContainer', 'engineOptions'],
      ['authPicker', 'engineAuthTrigger', 'engineAuthOptions']
    ]) {
      const picker = document.getElementById(pickerID);
      document.addEventListener('click', event => {
        if (!picker.contains(event.target)) closePicker(triggerID, optionsID);
      });
      picker.addEventListener('focusout', event => {
        if (!picker.contains(event.relatedTarget)) closePicker(triggerID, optionsID);
      });
    }
