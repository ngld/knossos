import { UseFormRegisterReturn } from 'react-hook-form';
import { InputGroup as OriginalInputGroup, InputGroupProps2, Checkbox as OriginalCheckbox, CheckboxProps, TextArea as OriginalTextArea, TextAreaProps, HTMLSelect as OriginalHTMLSelect, HTMLSelectProps } from '@blueprintjs/core';

export function InputGroup({
  fi,
  ...props
}: InputGroupProps2 & { fi: UseFormRegisterReturn }): React.ReactElement {
  const { ref, ...formProps } = fi;
  return <OriginalInputGroup {...props} inputRef={ref} {...formProps} />;
}

export function Checkbox({
  fi,
  ...props
}: CheckboxProps & { fi: UseFormRegisterReturn }): React.ReactElement {
  const { ref, ...formProps } = fi;
  return <OriginalCheckbox {...props} inputRef={ref} {...formProps} />;
}

export function TextArea({
  fi,
  ...props
}: TextAreaProps & { fi: UseFormRegisterReturn }): React.ReactElement {
  const { ref, ...formProps } = fi;
  return <OriginalTextArea {...props} inputRef={ref} {...formProps} />;
}

export function HTMLSelect({fi, ...props}: HTMLSelectProps & {fi:UseFormRegisterReturn}): React.ReactElement {
  return <OriginalHTMLSelect {...props} {...fi} />;
}
